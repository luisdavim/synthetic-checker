package checks

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/luisdavim/synthetic-checker/pkg/config"
)

func TestDnsCheck(t *testing.T) {
	type expected struct {
		ok  bool
		err *net.DNSError
	}
	tests := []struct {
		name     string
		config   config.DNSCheck
		expected expected
	}{
		{
			name: "OK",
			config: config.DNSCheck{
				Host: "www.google.com",
			},
			expected: expected{
				ok:  true,
				err: nil,
			},
		}, {
			name: "KO",
			config: config.DNSCheck{
				Host:               "fake-dns-name.fake.com",
				MinRequiredResults: 100,
			},
			expected: expected{
				ok:  false,
				err: &net.DNSError{Err: "no such host", Name: "fake-dns-name.fake.com", Server: "", IsTimeout: false, IsTemporary: false, IsNotFound: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewDNSCheck("test", tt.config)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			ok, err := c.Execute(context.TODO())
			if (err != nil && tt.expected.err != nil) && (!reflect.DeepEqual(err, tt.expected.err)) {
				t.Errorf("unexpected response error, wanted: %#v, got: %#v", tt.expected.err, err)
			}
			if ok != tt.expected.ok {
				t.Errorf("unexpected status, wanted: %t, got: %t", tt.expected.ok, ok)
			}
		})
	}
}
