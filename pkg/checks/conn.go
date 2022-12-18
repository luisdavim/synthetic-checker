package checks

import (
	"context"
	"fmt"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var _ api.Check = &connCheck{}

type connCheck struct {
	config *config.ConnCheck
	dialer *net.Dialer
}

// NewConnCheck returns a connectivity check for the given configuration
func NewConnCheck(name string, config config.ConnCheck) (api.Check, error) {
	if name == "" {
		return nil, fmt.Errorf("CheckName must not be empty")
	}
	if config.Address == "" {
		return nil, fmt.Errorf("address must not be empty")
	}
	if config.Protocol == "" {
		config.Protocol = "tcp"
	}
	if config.Interval.Duration == 0 {
		config.Interval = metav1.Duration{Duration: 30 * time.Second}
	}
	if config.Timeout.Duration == 0 {
		config.Timeout = metav1.Duration{Duration: time.Second}
	}

	return &connCheck{
		config: &config,
	}, nil
}

// Interval indicates how often the check should be performed
func (c *connCheck) Interval() metav1.Duration {
	return c.config.Interval
}

// InitialDelay indicates how long to delay the check start
func (c *connCheck) InitialDelay() metav1.Duration {
	return c.config.InitialDelay
}

// Execute performs the check
func (c *connCheck) Execute(ctx context.Context) (bool, error) {
	if c.dialer == nil {
		c.dialer = &net.Dialer{
			Timeout: c.config.Timeout.Duration,
		}
	}

	conn, err := c.dialer.DialContext(ctx, c.config.Protocol, c.config.Address)
	if err == nil {
		_ = conn.Close()
	}
	ok := err == nil
	if !ok {
		err = fmt.Errorf("failed to connect: %w", err)
	}
	return ok, err
}
