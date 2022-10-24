package checker

import (
	"context"
	"encoding/json"
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
	checks map[string]api.Check
	status map[string]api.Status
	log    zerolog.Logger
	sync.RWMutex
}

// NewFromConfig creates an re check runner from the given configuration
func NewFromConfig(cfg config.Config) (*CheckRunner, error) {
	prometheus.MustRegister(checkStatus, checkDuration)
	runner := &CheckRunner{
		checks: make(map[string]api.Check),
		status: make(map[string]api.Status),
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

	return runner, nil
}

// GetStatus returns the overall status of all the checks
func (runner *CheckRunner) GetStatus() map[string]api.Status {
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
}

// Run schedules all the checks, running them periodically in the background, according to their configuration
func (runner *CheckRunner) Run() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	runner.RunWithContext(ctx)
	return cancel
}

func (runner *CheckRunner) RunWithContext(ctx context.Context) {
	for name, check := range runner.checks {
		go func(name string, check api.Check) {
			ticker := time.NewTicker(check.Interval())
			for {
				select {
				case <-ticker.C:
					runner.check(ctx, name, check)
				case <-ctx.Done():
					runner.log.Info().Msgf("Stopping %s checks", name)
					ticker.Stop()
					return
				}
			}
		}(name, check)
	}
}

// Sync fetches the state from the leader
func (runner *CheckRunner) Sync(leader string) {
	res, err := http.Get("http://" + leader + ":8080/")
	if err != nil {
		runner.log.Err(err).Msg("failed to sync")
		return
	}
	defer res.Body.Close()
	status := make(map[string]api.Status)
	err = json.NewDecoder(res.Body).Decode(&status)
	if err != nil {
		runner.log.Err(err).Msg("failed to sync")
		return
	}

	for name, result := range status {
		runner.updateStatusFor(name, result)
	}
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
	checkDuration.With(prometheus.Labels{"name": name}).Observe(float64(status.Duration.Milliseconds()))
	if !status.OK {
		if status.ContiguousFailures == 0 {
			status.TimeOfFirstFailure = status.Timestamp
		}
		status.ContiguousFailures++
		checkStatus.With(prometheus.Labels{"name": name}).Set(0)
	} else {
		status.ContiguousFailures = 0
		checkStatus.With(prometheus.Labels{"name": name}).Set(1)
	}
	runner.log.Err(err).Bool("healthy", status.OK).Msgf("Check status for %s", name)
	runner.updateStatusFor(name, status)
}
