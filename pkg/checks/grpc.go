package checks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/alts"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var _ api.Check = &grpcCheck{}

type grpcCheck struct {
	config   *config.GRPCCheck
	dialOpts []grpc.DialOption
	callOpts []grpc.CallOption
}

func buildCredentials(skipVerify bool, caCerts, clientCert, clientKey, serverName string) (credentials.TransportCredentials, error) {
	var cfg tls.Config

	if clientCert != "" && clientKey != "" {
		keyPair, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load tls client cert/key pair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{keyPair}
	}

	if skipVerify {
		cfg.InsecureSkipVerify = true
	} else if caCerts != "" {
		// override system roots
		rootCAs := x509.NewCertPool()
		pem, err := os.ReadFile(caCerts)
		if err != nil {
			return nil, fmt.Errorf("failed to load root CA certificates from file (%s: %w", caCerts, err)
		}
		if !rootCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no root CA certs parsed from file %s", caCerts)
		}
		cfg.RootCAs = rootCAs
	}
	if serverName != "" {
		cfg.ServerName = serverName
	}

	return credentials.NewTLS(&cfg), nil
}

func NewGrpcCheck(name string, config config.GRPCCheck) (api.Check, error) {
	if name == "" {
		return nil, fmt.Errorf("CheckName must not be empty")
	}
	if config.Address == "" {
		return nil, fmt.Errorf("address must not be empty")
	}
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = time.Second
	}

	dOpts := []grpc.DialOption{
		grpc.WithUserAgent(config.UserAgent),
		grpc.WithBlock(),
	}
	cOpts := []grpc.CallOption{}

	if config.TLS {
		creds, err := buildCredentials(config.TLSNoVerify, config.TLSCACert, config.TLSClientCert, config.TLSClientKey, config.TLSServerName)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tls credentials: %w", err)
		}
		dOpts = append(dOpts, grpc.WithTransportCredentials(creds))
	} else if config.ALTS {
		creds := alts.NewServerCreds(alts.DefaultServerOptions())
		dOpts = append(dOpts, grpc.WithTransportCredentials(creds))
	} else if config.SPIFFE {
		spiffeCtx, spifCancel := context.WithTimeout(context.Background(), config.RPCTimeout)
		defer spifCancel()
		source, err := workloadapi.NewX509Source(spiffeCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tls credentials with spiffe: %w", err)
		}
		creds := credentials.NewTLS(tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeAny()))
		dOpts = append(dOpts, grpc.WithTransportCredentials(creds))
	} else {
		dOpts = append(dOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if config.GZIP {
		cOpts = append(cOpts, grpc.UseCompressor(gzip.Name))
	}

	return &grpcCheck{
		config:   &config,
		dialOpts: dOpts,
		callOpts: cOpts,
	}, nil
}

// Interval indicates how often the check should be performed
func (c *grpcCheck) Interval() time.Duration {
	return c.config.Interval
}

// InitialDelay indicates how long to delay the check start
func (c *grpcCheck) InitialDelay() time.Duration {
	return c.config.InitialDelay
}

// Execute performs the check
func (c *grpcCheck) Execute(ctx context.Context) (bool, error) {
	dialCtx, dialCancel := context.WithTimeout(context.Background(), c.config.ConnTimeout)
	defer dialCancel()
	conn, err := grpc.DialContext(dialCtx, c.config.Address, c.dialOpts...)
	if err != nil {
		return false, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), c.config.RPCTimeout)
	defer rpcCancel()
	rpcCtx = metadata.NewOutgoingContext(rpcCtx, c.config.RPCHeaders)
	resp, err := healthpb.NewHealthClient(conn).Check(rpcCtx,
		&healthpb.HealthCheckRequest{
			Service: c.config.Service,
		}, c.callOpts...)
	if err != nil {
		return false, fmt.Errorf("rpc call failed: %w", err)
	}

	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return false, fmt.Errorf("service unhealthy (responded with %q)", resp.GetStatus().String())
	}
	return true, nil
}
