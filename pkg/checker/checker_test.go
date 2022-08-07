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
			c.check(context.TODO(), checkName, c.checks[checkName])
			actual := c.status[checkName]
			if actual.OK != tt.expected.OK {
				t.Errorf("unexpected status, wanted: %t, got: %t", tt.expected.OK, actual.OK)
			}
			if actual.Error != tt.expected.Error {
				t.Errorf("unexpected error, wanted: %s, got: %s", tt.expected.Error, actual.Error)
			}
			if actual.ContiguousFailures != tt.expected.ContiguousFailures {
				t.Errorf("unexpected number of contiguous failures, wanted: %d, got: %d", tt.expected.ContiguousFailures, actual.ContiguousFailures)
			}
		})
	}
}
