package checker

import (
	"context"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var checkName string = "test"

func TestChecker(t *testing.T) {
	tests := []struct {
		name     string
		config   config.Config
		response http.Response
		expected api.Status
	}{
		{
			name: "Http OK",
			config: config.Config{
				HTTPChecks: map[string]config.HTTPCheck{
					checkName: {
						URL:    "http://fake.com/ok",
						Method: http.MethodGet,
					},
				},
			},
			response: http.Response{
				StatusCode: 200,
			},
			expected: api.Status{
				OK:    true,
				Error: "",
			},
		},
		{
			name: "Http Not OK",
			config: config.Config{
				HTTPChecks: map[string]config.HTTPCheck{
					checkName: {
						URL:    "http://fake.com/ok",
						Method: http.MethodGet,
					},
				},
			},
			response: http.Response{
				StatusCode: 500,
			},
			expected: api.Status{
				OK:                 false,
				Error:              "Unexpected status code: '500' expected: '200'",
				ContiguousFailures: 1,
			},
		},
		{
			name: "DNS OK",
			config: config.Config{
				DNSChecks: map[string]config.DNSCheck{
					checkName: {
						Host: "www.google.com",
					},
				},
			},
			expected: api.Status{
				OK: true,
			},
		},
		{
			name: "multiple OK",
			config: config.Config{
				DNSChecks: map[string]config.DNSCheck{
					checkName: {
						Host: "www.google.com",
					},
				},
				HTTPChecks: map[string]config.HTTPCheck{
					checkName: {
						URL:    "http://fake.com/ok",
						Method: http.MethodGet,
					},
				},
				ConnChecks: map[string]config.ConnCheck{
					checkName: {
						Address: "www.google.com:443",
					},
				},
			},
			response: http.Response{
				StatusCode: 200,
			},
			expected: api.Status{
				OK: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			httpmock.RegisterResponder(tt.config.HTTPChecks[checkName].Method, tt.config.HTTPChecks[checkName].URL, httpmock.ResponderFromResponse(&tt.response))
			c, err := NewFromConfig(tt.config)
			defer func() {
				// avoid panic with the prometheus.MustRegister used in NewFromConfig
				prometheus.Unregister(checkStatus)
				prometheus.Unregister(checkDuration)
			}()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			for name := range c.checks {
				c.check(context.TODO(), name, c.checks[name])
				actual, ok := c.GetStatusFor(name)
				if !ok {
					t.Errorf("missing status for %s", name)
				}
				if actual.OK != tt.expected.OK {
					t.Errorf("unexpected status, wanted: %t, got: %t", tt.expected.OK, actual.OK)
				}
				if actual.Error != tt.expected.Error {
					t.Errorf("unexpected error, wanted: %s, got: %s", tt.expected.Error, actual.Error)
				}
				if actual.ContiguousFailures != tt.expected.ContiguousFailures {
					t.Errorf("unexpected number of contiguous failures, wanted: %d, got: %d", tt.expected.ContiguousFailures, actual.ContiguousFailures)
				}
			}
		})
	}
}

func TestSync(t *testing.T) {
	tests := []struct {
		name     string
		config   config.Config
		status   string
		expected api.Status
	}{
		{
			name: "Http OK",
			config: config.Config{
				HTTPChecks: map[string]config.HTTPCheck{
					checkName: {
						URL:    "http://fake.com/ok",
						Method: http.MethodGet,
					},
				},
			},
			status: `{
    "test-http": {
        "ok": true,
        "timestamp": "2022-10-31T22:43:56.715843212Z",
        "duration": 918525104,
        "contiguousFailures": 0,
        "timeOfFirstFailure": "0001-01-01T00:00:00Z"
    }
}`,
			expected: api.Status{
				OK:    true,
				Error: "",
			},
		},
		{
			name: "Http Not OK",
			config: config.Config{
				HTTPChecks: map[string]config.HTTPCheck{
					checkName: {
						URL:    "http://fake.com/ok",
						Method: http.MethodGet,
					},
				},
			},
			status: `{
    "test-http": {
        "error": "Unexpected status code: '500' expected: '200'",
        "timestamp": "2022-10-31T22:43:56.715808368Z",
        "duration": 918794374,
        "contiguousFailures": 1,
        "timeOfFirstFailure": "2022-10-31T22:43:56.715808368Z"
    }
}`,
			expected: api.Status{
				OK:                 false,
				Error:              "Unexpected status code: '500' expected: '200'",
				ContiguousFailures: 1,
			},
		},
		{
			name: "DNS OK",
			config: config.Config{
				DNSChecks: map[string]config.DNSCheck{
					checkName: {
						Host: "www.google.com",
					},
				},
			},
			status: `{
    "test-dns": {
        "ok": true,
        "timestamp": "2022-10-31T22:43:56.715846962Z",
        "duration": 27122865,
        "contiguousFailures": 0,
        "timeOfFirstFailure": "0001-01-01T00:00:00Z"
    }
}`,
			expected: api.Status{
				OK: true,
			},
		},
		{
			name: "multiple OK",
			config: config.Config{
				DNSChecks: map[string]config.DNSCheck{
					checkName: {
						Host: "www.google.com",
					},
				},
				HTTPChecks: map[string]config.HTTPCheck{
					checkName: {
						URL:    "http://fake.com/ok",
						Method: http.MethodGet,
					},
				},
				ConnChecks: map[string]config.ConnCheck{
					checkName: {
						Address: "www.google.com:443",
					},
				},
			},
			status: `{
    "test-conn": {
        "ok": true,
        "timestamp": "2022-10-31T22:43:56.715798368Z",
        "duration": 267292,
        "contiguousFailures": 0,
        "timeOfFirstFailure": "0001-01-01T00:00:00Z"
    },
    "test-dns": {
        "ok": true,
        "timestamp": "2022-10-31T22:43:56.715846962Z",
        "duration": 27122865,
        "contiguousFailures": 0,
        "timeOfFirstFailure": "0001-01-01T00:00:00Z"
    },
    "test-http": {
        "ok": true,
        "timestamp": "2022-10-31T22:43:56.715843212Z",
        "duration": 918525104,
        "contiguousFailures": 0,
        "timeOfFirstFailure": "0001-01-01T00:00:00Z"
    }
}`,
			expected: api.Status{
				OK: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			httpmock.RegisterResponder(http.MethodGet, "http://leader:8080/", httpmock.NewStringResponder(http.StatusOK, tt.status))
			c, err := NewFromConfig(tt.config)
			defer func() {
				// avoid panic with the prometheus.MustRegister used in NewFromConfig
				prometheus.Unregister(checkStatus)
				prometheus.Unregister(checkDuration)
			}()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			sync := c.Syncer(false, 8080)
			sync("leader")
			for name := range c.checks {
				actual, ok := c.GetStatusFor(name)
				if !ok {
					t.Errorf("missing status for %s", name)
				}
				if actual.OK != tt.expected.OK {
					t.Errorf("unexpected status, wanted: %t, got: %t", tt.expected.OK, actual.OK)
				}
				if actual.Error != tt.expected.Error {
					t.Errorf("unexpected error, wanted: %s, got: %s", tt.expected.Error, actual.Error)
				}
				if actual.ContiguousFailures != tt.expected.ContiguousFailures {
					t.Errorf("unexpected number of contiguous failures, wanted: %d, got: %d", tt.expected.ContiguousFailures, actual.ContiguousFailures)
				}
			}
		})
	}
}
