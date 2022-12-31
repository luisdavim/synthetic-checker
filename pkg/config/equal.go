package config

import (
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func (c HTTPCheck) Equal(other HTTPCheck) bool {
	if c.URL != other.URL {
		return false
	}
	if c.Method != other.Method {
		return false
	}
	if c.Body != other.Body {
		return false
	}
	if c.ExpectedBody != other.ExpectedBody {
		return false
	}
	if c.ExpectedStatus != other.ExpectedStatus {
		return false
	}
	if c.CertExpiryThreshold != other.CertExpiryThreshold {
		return false
	}
	if c.BaseCheck != other.BaseCheck {
		return false
	}

	return maps.Equal(c.Headers, other.Headers)
}

func (c GRPCCheck) Equal(other GRPCCheck) bool {
	if c.Address != other.Address {
		return false
	}
	if c.Service != other.Service {
		return false
	}
	if c.UserAgent != other.UserAgent {
		return false
	}
	if c.ConnTimeout != other.ConnTimeout {
		return false
	}
	if c.RPCTimeout != other.RPCTimeout {
		return false
	}
	if c.TLS != other.TLS {
		return false
	}
	if c.TLSNoVerify != other.TLSNoVerify {
		return false
	}
	if c.TLSCACert != other.TLSCACert {
		return false
	}
	if c.TLSClientCert != other.TLSClientCert {
		return false
	}
	if c.TLSClientKey != other.TLSClientKey {
		return false
	}
	if c.TLSServerName != other.TLSServerName {
		return false
	}
	if c.ALTS != other.ALTS {
		return false
	}
	if c.GZIP != other.GZIP {
		return false
	}
	if c.SPIFFE != other.SPIFFE {
		return false
	}
	if c.BaseCheck != other.BaseCheck {
		return false
	}
	if len(c.RPCHeaders) != len(other.RPCHeaders) {
		return false
	}
	for k, v := range c.RPCHeaders {
		if w, ok := other.RPCHeaders[k]; !ok || !slices.Equal(v, w) {
			return false
		}
	}
	return true
}

func (c TLSCheck) Equal(other TLSCheck) bool {
	if c.Address != other.Address {
		return false
	}
	if c.ExpiryThreshold != other.ExpiryThreshold {
		return false
	}
	if c.InsecureSkipVerify != other.InsecureSkipVerify {
		return false
	}
	if c.SkipChainValidation != other.SkipChainValidation {
		return false
	}
	if c.BaseCheck != other.BaseCheck {
		return false
	}
	return slices.Equal(c.HostNames, other.HostNames)
}

func (c DNSCheck) Equal(other DNSCheck) bool {
	return c == other
}

func (c ConnCheck) Equal(other ConnCheck) bool {
	return c == other
}

func (c K8sCheck) Equal(other K8sCheck) bool {
	return c == other
}

func (c K8sPing) Equal(other K8sPing) bool {
	return c == other
}
