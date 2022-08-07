package checks

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"

	"github.com/luisdavim/synthetic-checker/pkg/config"
)

func TestHttpCheck(t *testing.T) {
	type expected struct {
		ok  bool
		err error
	}
	tests := []struct {
		name     string
		config   config.HTTPCheck
		expected expected
		response http.Response
	}{
		{
			name: "200 OK",
			config: config.HTTPCheck{
				URL:    "http://fake.com/ok",
				Method: http.MethodGet,
			},
			expected: expected{
				ok:  true,
				err: nil,
			},
			response: http.Response{
				StatusCode: 200,
			},
		},
		{
			name: "500 NOT OK",
			config: config.HTTPCheck{
				URL:    "http://fake.com/ok",
				Method: http.MethodGet,
			},
			expected: expected{
				ok: false,
				err: ErrorUnexpectedStatus{
					expected: 200,
					got:      500,
				},
			},
			response: http.Response{
				StatusCode: 500,
			},
		},
		{
			name: "500 OK",
			config: config.HTTPCheck{
				URL:            "http://fake.com/ok",
				Method:         http.MethodGet,
				ExpectedStatus: 500,
			},
			expected: expected{
				ok:  true,
				err: nil,
			},
			response: http.Response{
				StatusCode: 500,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			httpmock.RegisterResponder(tt.config.Method, tt.config.URL, httpmock.ResponderFromResponse(&tt.response))
			c, err := NewHTTPCheck("test", tt.config)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			ok, err := c.Execute(context.TODO())
			if !errors.Is(err, tt.expected.err) {
				t.Errorf("unexpected response error, wanted: %v, got: %v", tt.expected.err, err)
			}
			if ok != tt.expected.ok {
				t.Errorf("unexpected status, wanted: %t, got: %t", tt.expected.ok, ok)
			}
		})
	}
}
