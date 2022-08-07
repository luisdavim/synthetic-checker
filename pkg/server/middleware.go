package server

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

type httpReqInfo struct {
	// Time      string `json:"time,omitempty"`
	Method    string `json:"method,omitempty"`
	URI       string `json:"uri,omitempty"`
	Referer   string `json:"referer,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

func (ri httpReqInfo) MarshalZerologObject(e *zerolog.Event) {
	e.Str("method", ri.Method).
		Str("uri", ri.URI).
		// Str("time", ri.Time).
		Str("referer", ri.Referer).
		Str("user_agent", ri.UserAgent)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) logRequestHandler(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         200,
		}
		h.ServeHTTP(recorder, r)
		ri := &httpReqInfo{
			// Time:      time.Now().Format(time.RFC3339),
			Method:    r.Method,
			URI:       r.URL.String(),
			Referer:   r.Header.Get("Referer"),
			UserAgent: r.Header.Get("User-Agent"),
		}

		// HTTPReqInfo implements zerolog.LogArrayMarshaler
		s.logger.Info().Object("request", ri).Int("status", recorder.status).Msg("accessed")
	}
	return http.HandlerFunc(fn)
}

func instrumentedHandler(h http.HandlerFunc, name string) http.HandlerFunc {
	return promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": name}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, h),
	)
}

func removeTrailingSlash(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		h.ServeHTTP(w, r)
	})
}

func (s *Server) basicAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if s.config.Auth.User != user || s.config.Auth.Pass != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte("Unauthorized")); err != nil {
				s.logger.Err(err).Msg("failed to write response")
			}
			return
		}
		h.ServeHTTP(w, r)
	})
}
