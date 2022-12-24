package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/checks"
	"github.com/luisdavim/synthetic-checker/pkg/config"
	"github.com/luisdavim/synthetic-checker/pkg/informer"
)

var (
	checkCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "check_status_total",
		Help: "Number of check status occurences",
	}, []string{"name", "status"})

	checkStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "check_status_up",
		Help: "Status from the check",
	}, []string{"name"})

	checkDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "check_duration_ms",
		Help:    "Duration of the check",
		Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	}, []string{"name"})
)

// Runner reprents the main checks runner (checker)
// responsible for scheduling and executing (running) all the checks
type Runner struct {
	checks          api.Checks
	status          api.Statuses
	stop            map[string](chan struct{})
	log             zerolog.Logger
	informer        *informer.Informer
	upstreamRefresh time.Duration
	informOnly      bool
	sync.RWMutex
}

// NewFromConfig creates a check runner from the given configuration
func NewFromConfig(cfg config.Config, start bool) (*Runner, error) {
	prometheus.MustRegister(checkStatus, checkCount, checkDuration)
	r := &Runner{
		checks: make(api.Checks),
		status: make(api.Statuses),
		stop:   make(map[string](chan struct{})),
		log:    zerolog.New(os.Stderr).With().Timestamp().Str("name", "checker").Logger().Level(zerolog.InfoLevel),
	}

	if err := r.AddFromConfig(cfg, start); err != nil {
		return nil, err
	}

	if len(cfg.Informer.Upstreams) > 0 {
		var err error
		r.informer, err = informer.New(cfg.Informer.Upstreams)
		if err != nil {
			return nil, err
		}
		r.informOnly = cfg.Informer.InformOnly
		r.upstreamRefresh = cfg.Informer.RefreshInterval.Duration
		if r.upstreamRefresh == 0 {
			r.upstreamRefresh = 24 * time.Hour
		}
	}

	return r, nil
}

// AddFromConfig loads the checks from the given configuration
func (r *Runner) AddFromConfig(cfg config.Config, start bool) error {
	// setup HTTP checks
	for name, config := range cfg.HTTPChecks {
		check, err := checks.NewHTTPCheck(name, config)
		if err != nil {
			return err
		}
		r.AddCheck(name+"-http", check, start)
	}

	// setup DNS checks
	for name, config := range cfg.DNSChecks {
		check, err := checks.NewDNSCheck(name, config)
		if err != nil {
			return err
		}
		r.AddCheck(name+"-dns", check, start)
	}

	// setup K8s checks
	for name, config := range cfg.K8sChecks {
		check, err := checks.NewK8sCheck(name, config)
		if err != nil {
			return err
		}
		r.AddCheck(name+"-k8s", check, start)
	}

	// setup conn checks
	for name, config := range cfg.ConnChecks {
		check, err := checks.NewConnCheck(name, config)
		if err != nil {
			return err
		}
		r.AddCheck(name+"-conn", check, start)
	}

	// setup TLS checks
	for name, config := range cfg.TLSChecks {
		var err error
		r.checks[name+"-tls"], err = checks.NewTLSCheck(name, config)
		if err != nil {
			return err
		}
	}

	// setup gRPC checks
	for name, config := range cfg.GRPCChecks {
		check, err := checks.NewGrpcCheck(name, config)
		if err != nil {
			return err
		}
		r.AddCheck(name+"-grpc", check, start)
	}
	return nil
}

// AddCheck schedules a new check
func (r *Runner) AddCheck(name string, check api.Check, start bool) {
	if r.informOnly {
		start = false
	}
	r.log.Info().Str("name", name).Msg("new check")
	r.Lock()
	_, found := r.checks[name]
	r.checks[name] = check
	if !found && start {
		r.stop[name] = make(chan struct{})
		r.schedule(context.Background(), name)
	}
	r.Unlock()
	if r.informer != nil {
		err := r.informer.CreateOrUpdate(check)
		r.log.Err(err).Str("name", name).Msg("syncing check upstream")
	}
}

// DelCheck stops the given check and removes it from the running config
func (r *Runner) DelCheck(name string) {
	r.log.Info().Str("name", name).Msg("deleting check")
	r.Lock()
	if stopCh, ok := r.stop[name]; ok && stopCh != nil {
		r.log.Info().Str("name", name).Msg("stopping check")
		close(stopCh)
	}
	delete(r.stop, name)
	delete(r.checks, name)
	delete(r.status, name)
	r.Unlock()
	if r.informer != nil {
		err := r.informer.DeleteByName(name)
		r.log.Err(err).Str("name", name).Msg("deleting check upstream")
	}
}

// GetStatus returns the overall status of all the checks
func (r *Runner) GetStatus() api.Statuses {
	return r.status
}

// GetStatusFor returns the status for the given check
func (r *Runner) GetStatusFor(name string) (api.Status, bool) {
	r.RLock()
	n, ok := r.status[name]
	r.RUnlock()
	return n, ok
}

// updateStatusFor sets the status for the given check
func (r *Runner) updateStatusFor(name string, status api.Status) {
	r.Lock()
	r.status[name] = status
	r.Unlock()
	r.updateMetricsFor(name)
}

// updateMetricsFor generates Prometheus metrics from the status of the given check
func (r *Runner) updateMetricsFor(name string) {
	status, ok := r.GetStatusFor(name)
	if !ok {
		r.log.Warn().Str("name", name).Msg("status not found")
		return
	}
	var statusVal float64
	statusName := "error"
	if status.OK {
		statusName = "success"
		statusVal = 1
	}
	checkStatus.With(prometheus.Labels{"name": name}).Set(statusVal)
	checkCount.With(prometheus.Labels{"name": name, "status": statusName}).Inc()
	checkDuration.With(prometheus.Labels{"name": name}).Observe(float64(status.Duration.Milliseconds()))
}

// Start schedules all the checks, running them periodically in the background, according to their configuration
func (r *Runner) Start() context.CancelFunc {
	ctx, stop := context.WithCancel(context.Background())
	r.Run(ctx)
	return stop
}

// Run schedules all the checks, running them periodically in the background, according to their configuration
// in the informer is configured, it will also set up a refresher to ensure the configuration is eventually consistent, even if we miss update events
func (r *Runner) Run(ctx context.Context) {
	for name := range r.checks {
		if _, ok := r.stop[name]; ok {
			// already running
			continue
		}
		r.stop[name] = make(chan struct{})
		r.schedule(ctx, name)
	}
	if r.informer != nil {
		_ = r.upstreamRefresher(ctx)
	}
}

func (r *Runner) RefreshUpstreams() {
	for name, check := range r.checks {
		err := r.informer.Replace(check)
		r.log.Err(err).Str("name", name).Msg("syncing check upstream")
	}
}

func (r *Runner) upstreamRefresher(ctx context.Context) error {
	if r.informer == nil {
		return fmt.Errorf("missing informer configuration")
	}
	r.log.Info().Msg("starting upstream refresher")
	go func() {
		ticker := time.NewTicker(r.upstreamRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.RefreshUpstreams()
			case <-ctx.Done():
				r.log.Info().Msg("stopping upstream refresher")
				return
			}
		}
	}()

	return nil
}

// schedule executes the check on the configured interval
func (r *Runner) schedule(ctx context.Context, name string) {
	// ctx, _ = context.WithCancel(ctx)
	r.log.Info().Str("name", name).Msg("starting checks")
	go func() {
		time.Sleep(r.checks[name].InitialDelay().Duration)
		r.check(ctx, name)
		ticker := time.NewTicker(r.checks[name].Interval().Duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.check(ctx, name)
			case <-ctx.Done():
				r.log.Info().Str("name", name).Msg("stopping checks")
				return
			case <-r.stop[name]:
				r.log.Info().Str("name", name).Msg("got quit signal stopping checks")
				return
			}
		}
	}()
}

// Stop stops all checks
func (r *Runner) Stop() {
	for name := range r.checks {
		if stopCh, ok := r.stop[name]; ok && stopCh != nil {
			close(stopCh)
		}
		delete(r.stop, name)
	}
}

// Syncer returns a sync function that fetches the state from the leader
func (r *Runner) Syncer(useSSL bool, port int) func(string) {
	protocol := "http"
	if useSSL {
		protocol += "s"
	}
	return func(leader string) {
		res, err := http.Get(fmt.Sprintf("%s://%s:%d/", protocol, leader, port))
		if err != nil {
			r.log.Err(err).Msg("failed to sync")
			return
		}
		defer res.Body.Close()
		status := make(api.Statuses)
		err = json.NewDecoder(res.Body).Decode(&status)
		if err != nil {
			r.log.Err(err).Msg("failed to sync")
			return
		}

		for name, result := range status {
			r.updateStatusFor(name, result)
		}
		r.log.Info().Msg("synced data from leader")
	}
}

// Check runs all the checks in parallel and waits for them to complete
func (r *Runner) Check(ctx context.Context) {
	var wg sync.WaitGroup
	for name := range r.checks {
		wg.Add(1)
		go func(name string, check api.Check) {
			defer wg.Done()
			time.Sleep(check.InitialDelay().Duration)
			r.check(ctx, name)
		}(name, r.checks[name])
	}
	wg.Wait()
}

func (r *Runner) Summary() (allFailed, anyFailed bool) {
	status := r.GetStatus()
	return status.Evaluate()
}

// check executes one check and stores the resulting status
func (r *Runner) check(ctx context.Context, name string) {
	var err error
	status, _ := r.GetStatusFor(name)
	status.Error = ""
	status.Timestamp = time.Now()
	status.OK, err = r.checks[name].Execute(ctx)
	if err != nil {
		status.Error = err.Error()
	}
	status.Duration = metav1.Duration{Duration: time.Since(status.Timestamp)}
	if !status.OK {
		if status.ContiguousFailures == 0 {
			status.TimeOfFirstFailure = status.Timestamp
		}
		status.ContiguousFailures++
	} else {
		status.ContiguousFailures = 0
	}
	r.log.Err(err).Bool("healthy", status.OK).Str("name", name).Msg("check status")
	r.updateStatusFor(name, status)
}
