package checks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

// httpCheck represents an http checker
type httpCheck struct {
	config *config.HTTPCheck
	client *http.Client
}

// ErrorUnexpectedStatus is returned when the service being checked returns an unexpected status code
type ErrorUnexpectedStatus struct {
	expected int
	got      int
}

// Error makes ErrorUnexpectedStatus implement the error interface
func (e ErrorUnexpectedStatus) Error() string {
	return fmt.Sprintf("Unexpected status code: '%v' expected: '%v'", e.got, e.expected)
}

// NewHTTPCheck creates a new http check from the given configuration
func NewHTTPCheck(name string, config config.HTTPCheck) (api.Check, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL must not be empty")
	}
	if _, err := url.Parse(config.URL); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("CheckName must not be empty")
	}

	if config.ExpectedStatus == 0 {
		config.ExpectedStatus = http.StatusOK
	}
	if config.Method == "" {
		config.Method = http.MethodGet
	}
	if config.Timeout == 0 {
		config.Timeout = time.Second
	}

	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}

	check := &httpCheck{
		config: &config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
	return check, nil
}

// Interval indicates how often the check should be performed
func (c *httpCheck) Interval() time.Duration {
	return c.config.Interval
}

// Execute performs the check
func (c *httpCheck) Execute(ctx context.Context) (bool, error) {
	resp, err := c.do(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != c.config.ExpectedStatus {
		return false, ErrorUnexpectedStatus{
			got:      resp.StatusCode,
			expected: c.config.ExpectedStatus,
		}
	}

	if c.config.ExpectedBody != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("failed to read response body: %w", err)
		}

		if !strings.Contains(string(body), c.config.ExpectedBody) {
			return false, fmt.Errorf("body does not contain expected content '%v'", c.config.ExpectedBody)
		}
	}

	return true, nil
}

// do executes the HTTP request to the target URL
// It is the callers responsibility to close the response body
func (c *httpCheck) do(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, c.config.Method, c.config.URL, strings.NewReader(c.config.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for h, v := range c.config.Headers {
		req.Header.Add(h, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute '%v' request: %w", c.config.Method, err)
	}

	return resp, nil
}
