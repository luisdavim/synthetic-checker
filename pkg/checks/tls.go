package checks

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var _ api.Check = &tlsCheck{}

type tlsCheck struct {
	config  *config.TLSCheck
	tlsOpts *tls.Config
}

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
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = time.Second
	}
	if config.ExpiryThreshold == 0 {
		config.ExpiryThreshold = 7 * day
	}
	if len(config.HostNames) == 0 {
		config.HostNames = append(config.HostNames, host[0])
	}

	return &tlsCheck{
		config: &config,
		tlsOpts: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}, nil
}

// Interval indicates how often the check should be performed
func (c *tlsCheck) Interval() time.Duration {
	return c.config.Interval
}

// InitialDelay indicates how long to delay the check start
func (c *tlsCheck) InitialDelay() time.Duration {
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
		err = conn.VerifyHostname(hostName)
		if err != nil {
			return false, fmt.Errorf("hostname %s doesn't match with certificate: %w", hostName, err)
		}
	}

	if time.Now().Before(conn.ConnectionState().PeerCertificates[0].NotBefore) {
		return false, fmt.Errorf("the certificate is not yet valid")
	}

	ttl := time.Until(conn.ConnectionState().PeerCertificates[0].NotAfter)
	if ttl <= c.config.ExpiryThreshold {
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
