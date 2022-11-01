package config

import (
	"time"

	"google.golang.org/grpc/metadata"
)

type Config struct {
	HTTPChecks map[string]HTTPCheck `mapstructure:"httpChecks"`
	DNSChecks  map[string]DNSCheck  `mapstructure:"dnsChecks"`
	K8sChecks  map[string]K8sCheck  `mapstructure:"k8sChecks"`
	ConnChecks map[string]ConnCheck `mapstructure:"connChecks"`
	GRPCChecks map[string]GRPCCheck `mapstructure:"grpcChecks"`
}

// BaseCheck holds the common properties across checks
type BaseCheck struct {
	// Timeout is the timeout used for the check duration, defaults to "1s".
	Timeout time.Duration `mapstructure:"timeout,omitempty"`
	// Interval defines how often the check should be executed, defaults to 30 seconds.
	Interval time.Duration `mapstructure:"interval,omitempty"`
	// InitialDelay defines a time to wait for before starting the check
	InitialDelay time.Duration `mapstructure:"initialDelay,omitempty"`
}

// HTTPCheck configures a check for the response from a given URL.
// The only required field is `URL`, which must be a valid URL.
type HTTPCheck struct {
	// URL is the URL to  be checked.
	URL string `mapstructure:"url"`
	// Method is the HTTP method to use for this check.
	// Method is optional and defaults to `GET` if undefined.
	Method string `mapstructure:"method,omitempty"`
	// Headers to set on the request
	Headers map[string]string `mapstructure:"headers,omitempty"`
	// Body is an optional request body to be posted to the target URL.
	Body string `mapstructure:"body,omitempty"`
	// ExpectedStatus is the expected response status code, defaults to `200`.
	ExpectedStatus int `mapstructure:"expectedStatus,omitempty"`
	// ExpectedBody is optional; if defined, makes the check fail if the response body does not match
	ExpectedBody string `mapstructure:"expectedBody,omitempty"`
	BaseCheck
}

type GRPCCheck struct {
	Address       string        `mapstructure:"address,omitempty"`
	Service       string        `mapstructure:"service,omitempty"`
	UserAgent     string        `mapstructure:"userAgent,omitempty"`
	ConnTimeout   time.Duration `mapstructure:"connTimeout,omitempty"`
	RPCHeaders    metadata.MD   `mapstructure:"RPCHeaders,omitempty"`
	RPCTimeout    time.Duration `mapstructure:"rpcTimeout,omitempty"`
	TLS           bool          `mapstructure:"tls,omitempty"`
	TLSNoVerify   bool          `mapstructure:"tlsNoVerify,omitempty"`
	TLSCACert     string        `mapstructure:"tlscaCert,omitempty"`
	TLSClientCert string        `mapstructure:"tlsClientCert,omitempty"`
	TLSClientKey  string        `mapstructure:"tlsClientKey,omitempty"`
	TLSServerName string        `mapstructure:"tlsServerName,omitempty"`
	ALTS          bool          `mapstructure:"alts,omitempty"`
	Verbose       bool          `mapstructure:"verbose,omitempty"`
	GZIP          bool          `mapstructure:"gzip,omitempty"`
	SPIFFE        bool          `mapstructure:"spiffe,omitempty"`
	BaseCheck
}

type DNSCheck struct {
	// DNS name to check
	Host string `mapstructure:"host,omitempty"`
	// Minimum number of results the query must return, defaults to 1
	MinRequiredResults int `mapstructure:"minRequiredResults,omitempty"`
	BaseCheck
}

type ConnCheck struct {
	// AddressIP address or host to ping
	// see the net.Dial doccs for details
	Address string `mapstructure:"address,omitempty"`
	// Protocol to use, defaults to tcp
	// Known protocols are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only),
	// "udp", "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4"
	// (IPv4-only), "ip6" (IPv6-only), "unix", "unixgram" and
	// "unixpacket".
	// see the net.Dial doccs for details
	Protocol string `mapstructure:"protocol,omitempty"`
	BaseCheck
}

type K8sCheck struct {
	// Kind takes the common style of string which may be either `Kind.group.com` or `Kind.version.group.com`
	Kind string `mapstructure:"kind,omitempty"`
	// Namespace is the namespace where to look for the resource
	Namespace string `mapstructure:"namespace,omitempty"`
	// Name is the name of the resource
	Name string `mapstructure:"name,omitempty"`
	// LabelSelector comma separated list of key=value labels
	LabelSelector string `mapstructure:"labelSelector,omitempty"`
	// FieldSelector comma separated list of key=value fields
	FieldSelector string `mapstructure:"fieldSelector,omitempty"`
	BaseCheck
}
