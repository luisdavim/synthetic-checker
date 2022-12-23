package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	HealthEndpoint  = "/healthz"
	MetricsEndpoint = "/metrics"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Count of all HTTP requests",
	}, []string{"code", "method"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of all HTTP requests",
	}, []string{"code", "handler", "method"})
)

type ShutdownFunc func() error

type Server struct {
	router       *mux.Router
	logger       zerolog.Logger
	config       HTTP
	shutdownFunc ShutdownFunc
	stripSlashes bool
}

type Handler struct {
	// Methods adds a matcher for HTTP methods.
	// It accepts a sequence of one or more methods to be matched, e.g.:
	// "GET", "POST", "PUT".
	Methods []string
	// By default, if auth is enabled,
	// all endpoints, except the metrics and health,
	// will be secured
	NoAuth bool
	// By default all endpoints will be instrumented
	// and collect metrics regarding, duration, request count and status codes
	NoInstrumentation bool
	Name              string
	Func              http.HandlerFunc
}

type Routes map[string]Handler

// New creates a new instance of the server.
func New(cfg Config) *Server {
	r := mux.NewRouter()
	return NewWithRouter(cfg, r)
}

// NewWithRouter creates a new instance of the server
// and allows you to pass a router
func NewWithRouter(cfg Config, r *mux.Router) *Server {
	logLevel := zerolog.InfoLevel
	if cfg.Debug {
		logLevel = zerolog.DebugLevel
	}
	if r == nil {
		r = mux.NewRouter()
	}
	s := &Server{
		router:       r,
		config:       cfg.HTTP,
		stripSlashes: cfg.StripSlashes,
		logger:       zerolog.New(os.Stderr).With().Timestamp().Str("name", "serverLogger").Logger().Level(logLevel),
	}

	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)

	r.Handle(MetricsEndpoint, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	r.NotFoundHandler = instrumentedHandler(http.NotFoundHandler().ServeHTTP, "notFound")

	return s
}

// livelinessHandler is the default liveliness handler when one is not set by the user
func (s *Server) livelinessHandler(w http.ResponseWriter, r *http.Request) {
	s.JSONResponse(w, r, map[string]string{"status": "OK"}, http.StatusOK)
}

func (s *Server) setRoutes(r *mux.Router, routes Routes) {
	if _, ok := routes[HealthEndpoint]; !ok {
		routes[HealthEndpoint] = Handler{
			Func:    s.livelinessHandler,
			Methods: []string{"GET"},
			Name:    "root",
		}
	}
	for u, h := range routes {
		if !h.NoInstrumentation {
			// instrumentation was not explicitly disabled
			h.Func = instrumentedHandler(h.Func, h.Name)
		}
		if h.NoAuth || u == HealthEndpoint || u == MetricsEndpoint {
			// auth was explicitly disabled or this is a well known open endpoint
			r.Handle(u, h.Func).Methods(h.Methods...)
			continue
		}
		switch {
		case s.config.Auth.User != "" && s.config.Auth.Pass != "":
			r.Handle(u, s.basicAuth(h.Func)).Methods(h.Methods...)
		default:
			r.Handle(u, h.Func).Methods(h.Methods...)
		}
	}
}

// WithRoutes adds routes to the server, it may be called multiple times
// with different sets of routes.
func (s *Server) WithRoutes(routes Routes) {
	s.setRoutes(s.router, routes)
}

// WithPrefixedRoutes adds routes to the server under the prefix using a subrouter
func (s *Server) WithPrefixedRoutes(prefix string, routes Routes) {
	s.setRoutes(s.router.PathPrefix(prefix).Subrouter(), routes)
}

// WithShutdownFunc allows you to define the shutdown behavior.
// the ShutdownFunc will be called immediately before terminating the server
func (s *Server) WithShutdownFunc(shutdownFunc ShutdownFunc) {
	s.shutdownFunc = shutdownFunc
}

// WithNotFoundHandler allows you to specify a handler to use when no routes can be matched
func (s *Server) WithNotFoundHandler(notFound http.HandlerFunc) {
	s.router.NotFoundHandler = instrumentedHandler(notFound, "notfound")
}

// WithMethodNotAllowedHandler allows you to specify a handler to use when a method is not allowed for the matched route
func (s *Server) WithMethodNotAllowedHandler(notAllowed http.HandlerFunc) {
	s.router.MethodNotAllowedHandler = instrumentedHandler(notAllowed, "methodNotAllowed")
}

// Use appends a MiddlewareFunc to the chain.
// Middleware can be used to intercept or otherwise modify requests and/or responses,
// and are executed in the order that they are applied to the Router.
func (s *Server) Use(mwf ...mux.MiddlewareFunc) {
	s.router.Use(mwf...)
}

// Run starts the server and waits for the signal to stop
func (s *Server) Run() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	srvs := s.Start(signals)
	<-signals
	if s.shutdownFunc != nil {
		if err := s.shutdownFunc(); err != nil {
			s.logger.Err(err).Msg("failed to shutdown properly")
			os.Exit(1)
		}
	}
	for _, srv := range srvs {
		if err := srv.Shutdown(context.Background()); err != nil {
			s.logger.Err(err).Msg("failed to shutdown properly")
			os.Exit(1)
		}
	}
	os.Exit(0)
}

// Start starts the server in the background.
func (s *Server) Start(signals chan os.Signal) (srvs []*http.Server) {
	var rtr http.Handler
	rtr = s.router
	if s.stripSlashes {
		rtr = removeTrailingSlash(rtr)
	}
	if s.config.RequestLimit > 0 {
		lmt := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
		lmt.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"})
		rtr = tollbooth.LimitHandler(lmt, rtr)
	}
	rtr = s.logRequestHandler(rtr)
	go func() {
		srv := &http.Server{Addr: fmt.Sprintf(":%d", s.config.Port), Handler: rtr}
		if err := srv.ListenAndServe(); err != nil {
			s.logger.Err(err).Msg("Failed to start HTTP server")
			signals <- syscall.SIGABRT
		} else {
			srvs = append(srvs, srv)
		}
	}()
	go func() {
		if s.config.CertFile == "" || s.config.KeyFile == "" {
			return
		}
		srv := &http.Server{Addr: fmt.Sprintf(":%d", s.config.SecurePort), Handler: rtr}
		if err := srv.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile); err != nil {
			s.logger.Err(err).Msg("Failed to start HTTPS server")
			signals <- syscall.SIGABRT
		} else {
			srvs = append(srvs, srv)
		}
	}()
	return srvs
}
