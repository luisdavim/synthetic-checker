<!-- Code generated by gomarkdoc. DO NOT EDIT -->

# config

```go
import "github.com/luisdavim/synthetic-checker/pkg/config"
```

## Index

- [type BaseCheck](<#type-basecheck>)
- [type Config](<#type-config>)
- [type ConnCheck](<#type-conncheck>)
  - [func (c ConnCheck) Equal(other ConnCheck) bool](<#func-conncheck-equal>)
- [type DNSCheck](<#type-dnscheck>)
  - [func (c DNSCheck) Equal(other DNSCheck) bool](<#func-dnscheck-equal>)
- [type GRPCCheck](<#type-grpccheck>)
  - [func (c GRPCCheck) Equal(other GRPCCheck) bool](<#func-grpccheck-equal>)
- [type HTTPCheck](<#type-httpcheck>)
  - [func (c HTTPCheck) Equal(other HTTPCheck) bool](<#func-httpcheck-equal>)
- [type InformerCfg](<#type-informercfg>)
- [type K8sCheck](<#type-k8scheck>)
  - [func (c K8sCheck) Equal(other K8sCheck) bool](<#func-k8scheck-equal>)
- [type K8sPing](<#type-k8sping>)
  - [func (c K8sPing) Equal(other K8sPing) bool](<#func-k8sping-equal>)
- [type TLSCheck](<#type-tlscheck>)
  - [func (c TLSCheck) Equal(other TLSCheck) bool](<#func-tlscheck-equal>)
- [type Upstream](<#type-upstream>)


## type [BaseCheck](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L39-L46>)

BaseCheck holds the common properties across checks

```go
type BaseCheck struct {
    // Timeout is the timeout used for the check duration, defaults to "1s".
    Timeout metav1.Duration `mapstructure:"timeout,omitempty"`
    // Interval defines how often the check should be executed, defaults to 30 seconds.
    Interval metav1.Duration `mapstructure:"interval,omitempty"`
    // InitialDelay defines a time to wait for before starting the check
    InitialDelay metav1.Duration `mapstructure:"initialDelay,omitempty"`
}
```

## type [Config](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L9-L18>)

Config represents the checks configuration

```go
type Config struct {
    Informer   InformerCfg          `mapstructure:"informer,omitempty"`
    HTTPChecks map[string]HTTPCheck `mapstructure:"httpChecks"`
    GRPCChecks map[string]GRPCCheck `mapstructure:"grpcChecks"`
    DNSChecks  map[string]DNSCheck  `mapstructure:"dnsChecks"`
    ConnChecks map[string]ConnCheck `mapstructure:"connChecks"`
    TLSChecks  map[string]TLSCheck  `mapstructure:"tlsChecks"`
    K8sChecks  map[string]K8sCheck  `mapstructure:"k8sChecks"`
    K8sPings   map[string]K8sPing   `mapstructure:"k8sPings"`
}
```

## type [ConnCheck](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L131-L143>)

ConnCheck configures a conntivity check

```go
type ConnCheck struct {
    // Address is the IP address or host and port to ping
    // see the net.Dial docs for details
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
```

### func \(ConnCheck\) [Equal](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/equal.go#L114>)

```go
func (c ConnCheck) Equal(other ConnCheck) bool
```

## type [DNSCheck](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L122-L128>)

DNSCheck configures a probe to check if a DNS record resolves

```go
type DNSCheck struct {
    // DNS name to check
    Host string `mapstructure:"host,omitempty"`
    // Minimum number of results the query must return, defaults to 1
    MinRequiredResults int `mapstructure:"minRequiredResults,omitempty"`
    BaseCheck
}
```

### func \(DNSCheck\) [Equal](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/equal.go#L110>)

```go
func (c DNSCheck) Equal(other DNSCheck) bool
```

## type [GRPCCheck](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L70-L102>)

GRPCCheck configures a gRPC health check probe

```go
type GRPCCheck struct {
    // Address is the IP address or host to connect to
    Address string `mapstructure:"address,omitempty"`
    // Service name to check
    Service string `mapstructure:"service,omitempty"`
    // UserAgent defines the user-agent header value of health check requests
    UserAgent string `mapstructure:"userAgent,omitempty"`
    // ConnTimeout is the timeout for establishing connection
    ConnTimeout metav1.Duration `mapstructure:"connTimeout,omitempty"`
    // RPCHeaders sends metadata in the RPC request context
    RPCHeaders metadata.MD `mapstructure:"RPCHeaders,omitempty"`
    // RPCTimeout is the timeout for health check rpc
    RPCTimeout metav1.Duration `mapstructure:"rpcTimeout,omitempty"`
    // TLS indicates whether TLS should be used
    TLS bool `mapstructure:"tls,omitempty"`
    // TLSNoVerify makes the check skip the cert validation
    TLSNoVerify bool `mapstructure:"tlsNoVerify,omitempty"`
    // TLSCACert is the path to file containing CA certificates
    TLSCACert string `mapstructure:"tlscaCert,omitempty"`
    // TLSClientCert is the client certificate for authenticating to the server
    TLSClientCert string `mapstructure:"tlsClientCert,omitempty"`
    // TLSClientKey is the private key for for authenticating to the server
    TLSClientKey string `mapstructure:"tlsClientKey,omitempty"`
    // TLSServerName overrides the hostname used to verify the server certificate
    TLSServerName string `mapstructure:"tlsServerName,omitempty"`
    // ALTS indicates whether ALTS transport should be used
    ALTS bool `mapstructure:"alts,omitempty"`
    // GZIP indicates whether to use GZIPCompressor for requests and GZIPDecompressor for response
    GZIP bool `mapstructure:"gzip,omitempty"`
    // SPIFFE indicates if SPIFFE Workload API should be used to retrieve TLS credentials
    SPIFFE bool `mapstructure:"spiffe,omitempty"`
    BaseCheck
}
```

### func \(GRPCCheck\) [Equal](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/equal.go#L34>)

```go
func (c GRPCCheck) Equal(other GRPCCheck) bool
```

## type [HTTPCheck](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L50-L67>)

HTTPCheck configures a check for the response from a given URL. The only required field is \`URL\`, which must be a valid URL.

```go
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
    // CertExpiryThreshold is the minimum amount of time that the TLS certificate should be valid for
    CertExpiryThreshold metav1.Duration `mapstructure:"expiryThreshold,omitempty"`
    BaseCheck
}
```

### func \(HTTPCheck\) [Equal](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/equal.go#L8>)

```go
func (c HTTPCheck) Equal(other HTTPCheck) bool
```

## type [InformerCfg](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L20-L28>)

```go
type InformerCfg struct {
    // InformOnly, when set to true, will prevent the checks from being executed in the local instance
    InformOnly bool `json:"informOnly,omitempty"`
    // RefreshInterval indicates how often the checks will be refreshed upstream.
    // checks are pushed upstream when they are created or updated, this help keeping the system level-triggered
    // it defaults to 24h and should not be done too frequently.
    RefreshInterval metav1.Duration `json:"syncInterval,omitempty"`
    Upstreams       []Upstream      `mapstructure:"upstreams,omitempty"`
}
```

## type [K8sCheck](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L147-L159>)

K8sCheck configures a check that probes the status of a Kubernetes resource. It supports any resource type that uses standard k8s status conditions.

```go
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
```

### func \(K8sCheck\) [Equal](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/equal.go#L118>)

```go
func (c K8sCheck) Equal(other K8sCheck) bool
```

## type [K8sPing](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L162-L173>)

K8sPing is a conntivity check that will try to connect to all Pods matching the selector

```go
type K8sPing struct {
    // Namespace is the namespace where to look for the resource
    Namespace string `mapstructure:"namespace,omitempty"`
    // LabelSelector comma separated list of key=value labels
    LabelSelector string `mapstructure:"labelSelector,omitempty"`
    // Protocol to use, defaults to tcp
    // see the net.Dial doccs for details
    Protocol string `mapstructure:"protocol,omitempty"`
    // Port to ping
    Port int `mapstructure:"port,omitempty"`
    BaseCheck
}
```

### func \(K8sPing\) [Equal](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/equal.go#L122>)

```go
func (c K8sPing) Equal(other K8sPing) bool
```

## type [TLSCheck](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L105-L119>)

TLSCheck configures a TLS connection check, including certificate validation

```go
type TLSCheck struct {
    // Address is the IP address or host to connect to
    Address string `mapstructure:"address,omitempty"`
    // HostNames is a list of host names that the certificate should be valid for
    // defaults to the value of Address
    HostNames []string `mapstructure:"hostNames,omitempty"`
    // ExpiryThreshold is the minimum amount of time that the certificate should be valid for
    // defaults to 168h (7 days)
    ExpiryThreshold metav1.Duration `mapstructure:"expiryThreshold,omitempty"`
    // InsecureSkipVerify indicates whether the certificate should be checked when establishing the connection
    InsecureSkipVerify bool `mapstructure:"insecureSkipVerify"`
    // SkipChainValidation limita the certificate validation to the leaf certificate
    SkipChainValidation bool `mapstructure:"skipChainValidation,omitempty"`
    BaseCheck
}
```

### func \(TLSCheck\) [Equal](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/equal.go#L91>)

```go
func (c TLSCheck) Equal(other TLSCheck) bool
```

## type [Upstream](<https://github.com/luisdavim/synthetic-checker/blob/main/pkg/config/config.go#L32-L36>)

Upstream represents an upstream synthetic\-checker where to push checks to. This is useful when combined with the insgress watcher to generate remote checks for the local cluster

```go
type Upstream struct {
    URL     string            `mapstructure:"url"`
    Headers map[string]string `mapstructure:"headers,omitempty"`
    Timeout metav1.Duration   `mapstructure:"timeout,omitempty"`
}
```



Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
