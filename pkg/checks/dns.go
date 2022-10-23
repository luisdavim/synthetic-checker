package checks

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

type dnsCheck struct {
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
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = time.Second
	}
	if config.MinRequiredResults == 0 {
		config.MinRequiredResults = 1
	}

	return &dnsCheck{
		config: &config,
	}, nil
}

// Interval indicates how often the check should be performed
func (c *dnsCheck) Interval() time.Duration {
	return c.config.Interval
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
	ok := len(addrs) >= c.config.MinRequiredResults)
	if !ok {
		err = fmt.Errorf("insufficient number of results: %d < %d", len(addrs), c.config.MinRequiredResults)
	}
	return ok, err
}
