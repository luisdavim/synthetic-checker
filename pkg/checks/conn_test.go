package checks

import (
	"context"
	"testing"

	"github.com/luisdavim/synthetic-checker/pkg/config"
)

func TestConnCheck(t *testing.T) {
	type expected struct {
		ok  bool
		err error
	}
	tests := []struct {
		name     string
		config   config.ConnCheck
		expected expected
	}{
		{
			name: "udp OK",
			config: config.ConnCheck{
				Address:  "8.8.8.8:53",
				Protocol: "udp",
			},
			expected: expected{
				ok: true,
			},
		},
		{
			name: "tcp OK",
			config: config.ConnCheck{
				Address: "www.example.com:443",
			},
			expected: expected{
				ok: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewConnCheck("test", tt.config)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			ok, err := c.Execute(context.TODO())
			if err != nil && tt.expected.err == nil {
				t.Errorf("unexpected error: %v", err)
			}
			if (err != nil && tt.expected.err != nil) && (err.Error() != tt.expected.err.Error()) {
				t.Errorf("unexpected error, wanted: %v, got: %v", tt.expected.err, err)
			}
			if ok != tt.expected.ok {
				t.Errorf("unexpected status, wanted: %t, got: %t", tt.expected.ok, ok)
			}
		})
	}
}
