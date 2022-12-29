package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var _ api.Check = &dnsCheck{}

type dnsCheck struct {
	name     string
	config   *config.DNSCheck
	resolver *net.Resolver
}

// NewDNSCheck returns a Check that makes sure the configured hosts can be resolved
// to at least `MinRequiredResults` result, within the timeout specified by the provided context.
func NewDNSCheck(name string, config config.DNSCheck) (api.Check, error) {
	if name == "" {
		return nil, fmt.Errorf("CheckName must not be empty")
	}
	if config.Host == "" {
		return nil, fmt.Errorf("host must not be empty")
	}
	if config.Interval.Duration == 0 {
		config.Interval = metav1.Duration{Duration: 30 * time.Second}
	}
	if config.Timeout.Duration == 0 {
		config.Timeout = metav1.Duration{Duration: time.Second}
	}
	if config.MinRequiredResults == 0 {
		config.MinRequiredResults = 1
	}

	return &dnsCheck{
		name:   name,
		config: &config,
	}, nil
}

func (c *dnsCheck) Equal(other *dnsCheck) bool {
	return c.config.Equal(*other.config)
}

func (c *dnsCheck) Config() (string, string, string, error) {
	b, err := json.Marshal(c.config)
	if err != nil {
		return "", "", "", err
	}
	return "dns", c.name, string(b), nil
}

// Interval indicates how often the check should be performed
func (c *dnsCheck) Interval() metav1.Duration {
	return c.config.Interval
}

// InitialDelay indicates how long to delay the check start
func (c *dnsCheck) InitialDelay() metav1.Duration {
	return c.config.InitialDelay
}

// Execute performs the check
func (c *dnsCheck) Execute(ctx context.Context) (bool, error) {
	if c.resolver == nil {
		c.resolver = net.DefaultResolver
	}

	addrs, err := c.resolver.LookupHost(ctx, c.config.Host)
	if err != nil {
		return false, err
	}
	ok := len(addrs) >= c.config.MinRequiredResults
	if !ok {
		err = fmt.Errorf("insufficient number of results: %d < %d", len(addrs), c.config.MinRequiredResults)
	}
	return ok, err
}
