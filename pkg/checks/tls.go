package checks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var _ api.Check = &tlsCheck{}

type tlsCheck struct {
	name    string
	config  *config.TLSCheck
	tlsOpts *tls.Config
}

// NewTLSCheck returns a TLS connectivity check
// that validates is the address is reachable and presents a valid certificate
// it will also verify if the certificate is about to expire
func NewTLSCheck(name string, config config.TLSCheck) (api.Check, error) {
	if name == "" {
		return nil, fmt.Errorf("CheckName must not be empty")
	}
	if config.Address == "" {
		return nil, fmt.Errorf("address must not be empty")
	}
	host := strings.Split(config.Address, ":")
	if len(host) == 1 {
		config.Address += ":443"
	}
	if config.Interval.Duration == 0 {
		config.Interval = metav1.Duration{Duration: 30 * time.Second}
	}
	if config.Timeout.Duration == 0 {
		config.Timeout = metav1.Duration{Duration: time.Second}
	}
	if config.ExpiryThreshold.Duration == 0 {
		config.ExpiryThreshold = metav1.Duration{Duration: 7 * day}
	}
	if len(config.HostNames) == 0 {
		config.HostNames = append(config.HostNames, host[0])
	}

	return &tlsCheck{
		name:   name,
		config: &config,
		tlsOpts: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}, nil
}

func (c *tlsCheck) Equal(other *tlsCheck) bool {
	return c.config.Equal(*other.config)
}

func (c *tlsCheck) Config() (string, string, string, error) {
	b, err := json.Marshal(c.config)
	if err != nil {
		return "", "", "", err
	}
	return "tls", c.name, string(b), nil
}

// Interval indicates how often the check should be performed
func (c *tlsCheck) Interval() metav1.Duration {
	return c.config.Interval
}

// InitialDelay indicates how long to delay the check start
func (c *tlsCheck) InitialDelay() metav1.Duration {
	return c.config.InitialDelay
}

// Execute performs the check
func (c *tlsCheck) Execute(ctx context.Context) (bool, error) {
	conn, err := tls.Dial("tcp", c.config.Address, c.tlsOpts)
	if err != nil {
		return false, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	for _, hostName := range c.config.HostNames {
		if c.config.SkipChainValidation {
			err = conn.ConnectionState().PeerCertificates[0].VerifyHostname(hostName)
		} else {
			err = conn.VerifyHostname(hostName)
		}
		if err != nil {
			return false, fmt.Errorf("hostname %s doesn't match with certificate: %w", hostName, err)
		}
	}

	if time.Now().Before(conn.ConnectionState().PeerCertificates[0].NotBefore) {
		return false, fmt.Errorf("the certificate is not yet valid")
	}

	ttl := time.Until(conn.ConnectionState().PeerCertificates[0].NotAfter)
	if ttl <= c.config.ExpiryThreshold.Duration {
		return false, fmt.Errorf("the certificate will expire in %s", humanDuration(ttl))
	}

	// certs := conn.ConnectionState().PeerCertificates
	// for _, cert := range certs {
	// 	fmt.Printf("Issuer Name: %s\n", cert.Issuer)
	// 	fmt.Printf("Valid from: %s \n", cert.NotBefore.Format(time.RFC3339))
	// 	fmt.Printf("Expiry: %s \n", cert.NotAfter.Format(time.RFC3339))
	// 	fmt.Printf("Common Name: %s \n", cert.Issuer.CommonName)
	// }

	return true, nil
}
