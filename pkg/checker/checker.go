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

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/checks"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var (
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

// CheckRunner reprents the main checker responsible for executing all the checks
type CheckRunner struct {
	checks api.Checks
	status api.Statuses
	stop   map[string](chan struct{})
	log    zerolog.Logger
	sync.RWMutex
}

// NewFromConfig creates an re check runner from the given configuration
func NewFromConfig(cfg config.Config) (*CheckRunner, error) {
	prometheus.MustRegister(checkStatus, checkDuration)
	runner := &CheckRunner{
		checks: make(api.Checks),
		status: make(api.Statuses),
		stop:   make(map[string](chan struct{})),
		log:    zerolog.New(os.Stderr).With().Timestamp().Str("name", "checkerLogger").Logger().Level(zerolog.InfoLevel),
	}

	// setup HTTP checks
	for name, config := range cfg.HTTPChecks {
		var err error
		runner.checks[name+"-http"], err = checks.NewHTTPCheck(name, config)
		if err != nil {
			return nil, err
		}
	}

	// setup DNS checks
	for name, config := range cfg.DNSChecks {
		var err error
		runner.checks[name+"-dns"], err = checks.NewDNSCheck(name, config)
		if err != nil {
			return nil, err
		}
	}

	// setup K8s checks
	for name, config := range cfg.K8sChecks {
		var err error
		runner.checks[name+"-k8s"], err = checks.NewK8sCheck(name, config)
		if err != nil {
			return nil, err
		}
	}

	// setup conn checks
	for name, config := range cfg.ConnChecks {
		var err error
		runner.checks[name+"-conn"], err = checks.NewConnCheck(name, config)
		if err != nil {
			return nil, err
		}
	}

	// setup gRPC checks
	for name, config := range cfg.GRPCChecks {
		var err error
		runner.checks[name+"-grpc"], err = checks.NewGrpcCheck(name, config)
		if err != nil {
			return nil, err
		}
	}

	return runner, nil
}

// AddCheck schedules a new check
func (runner *CheckRunner) AddCheck(name string, check api.Check) {
	runner.Lock()
	if stopCh, ok := runner.stop[name]; ok {
		stopCh <- struct{}{}
		close(stopCh)
	}
	runner.checks[name] = check
	runner.stop[name] = make(chan struct{})
	runner.run(context.Background(), name, check, runner.stop[name])
	runner.Unlock()
}

// DelCheck stops the given check
func (runner *CheckRunner) DelCheck(name string) {
	runner.Lock()
	if stopCh, ok := runner.stop[name]; ok {
		stopCh <- struct{}{}
		close(stopCh)
	}
	delete(runner.stop, name)
	delete(runner.checks, name)
	delete(runner.status, name)
	runner.Unlock()
}

// GetStatus returns the overall status of all the checks
func (runner *CheckRunner) GetStatus() api.Statuses {
	return runner.status
}

// GetStatusFor returns the status for the given check
func (runner *CheckRunner) GetStatusFor(name string) (api.Status, bool) {
	runner.RLock()
	r, ok := runner.status[name]
	runner.RUnlock()
	return r, ok
}

// updateStatusFor sets the status for the given check
func (runner *CheckRunner) updateStatusFor(name string, r api.Status) {
	runner.Lock()
	runner.status[name] = r
	runner.Unlock()
	runner.updateMetricsFor(name)
}

// updateMetricsFor generates Prometheus metrics from the status of the given check
func (runner *CheckRunner) updateMetricsFor(name string) {
	status, ok := runner.GetStatusFor(name)
	if !ok {
		runner.log.Warn().Str("name", name).Msg("status not found")
		return
	}
	checkDuration.With(prometheus.Labels{"name": name}).Observe(float64(status.Duration.Milliseconds()))
	if status.OK {
		checkStatus.With(prometheus.Labels{"name": name}).Set(1)
	} else {
		checkStatus.With(prometheus.Labels{"name": name}).Set(0)
	}
}

// Start schedules all the checks, running them periodically in the background, according to their configuration
func (runner *CheckRunner) Start() context.CancelFunc {
	ctx, stop := context.WithCancel(context.Background())
	runner.Run(ctx)
	return stop
}

// Run schedules all the checks, running them periodically in the background, according to their configuration
func (runner *CheckRunner) Run(ctx context.Context) {
	for name, check := range runner.checks {
		runner.stop[name] = make(chan struct{})
		runner.run(ctx, name, check, runner.stop[name])
	}
}

func (runner *CheckRunner) run(ctx context.Context, name string, check api.Check, quit <-chan struct{}) {
	go func() {
		time.Sleep(check.InitialDelay())
		runner.check(ctx, name, check)
		ticker := time.NewTicker(check.Interval())
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runner.check(ctx, name, check)
			case <-ctx.Done():
				runner.log.Info().Str("name", name).Msg("stopping checks")
				return
			case <-quit:
				runner.log.Info().Str("name", name).Msg("got quit signal stopping checks")
				return
			}
		}
	}()
}

// Stop stops all checks
func (runner *CheckRunner) Stop() {
	for name := range runner.checks {
		if stopCh, ok := runner.stop[name]; ok {
			stopCh <- struct{}{}
			close(stopCh)
		}
		delete(runner.stop, name)
	}
}

// Syncer returns a sync function that fetches the state from the leader
func (runner *CheckRunner) Syncer(useSSL bool, port int) func(string) {
	protocol := "http"
	if useSSL {
		protocol += "s"
	}
	return func(leader string) {
		res, err := http.Get(fmt.Sprintf("%s://%s:%d/", protocol, leader, port))
		if err != nil {
			runner.log.Err(err).Msg("failed to sync")
			return
		}
		defer res.Body.Close()
		status := make(api.Statuses)
		err = json.NewDecoder(res.Body).Decode(&status)
		if err != nil {
			runner.log.Err(err).Msg("failed to sync")
			return
		}

		for name, result := range status {
			runner.updateStatusFor(name, result)
		}
		runner.log.Info().Msg("synced data from leader")
	}
}

// Check runs all the checks in parallel and waits for them to complete
func (runner *CheckRunner) Check(ctx context.Context) {
	var wg sync.WaitGroup
	for name, check := range runner.checks {
		wg.Add(1)
		go func(name string, check api.Check) {
			defer wg.Done()
			time.Sleep(check.InitialDelay())
			runner.check(ctx, name, check)
		}(name, check)
	}
	wg.Wait()
}

func (runner *CheckRunner) Summary() (allFailed, anyFailed bool) {
	status := runner.GetStatus()
	return Evaluate(status)
}

func Evaluate(status api.Statuses) (allFailed, anyFailed bool) {
	allFailed = true
	for _, result := range status {
		if !result.OK {
			anyFailed = true
		} else {
			allFailed = false
		}
	}
	return
}

// check executes one check and stores the resulting status
func (runner *CheckRunner) check(ctx context.Context, name string, check api.Check) {
	var err error
	status, _ := runner.GetStatusFor(name)
	status.Error = ""
	status.Timestamp = time.Now()
	status.OK, err = check.Execute(ctx)
	if err != nil {
		status.Error = err.Error()
	}
	status.Duration = time.Since(status.Timestamp)
	if !status.OK {
		if status.ContiguousFailures == 0 {
			status.TimeOfFirstFailure = status.Timestamp
		}
		status.ContiguousFailures++
	} else {
		status.ContiguousFailures = 0
	}
	runner.log.Err(err).Bool("healthy", status.OK).Str("name", name).Msg("check status")
	runner.updateStatusFor(name, status)
}
