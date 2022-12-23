package informer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

// Informer allows syncing check configuration to upstream synthetic-checkers
type Informer struct {
	config []config.Upstream
	client *http.Client
}

// New creates a new Informer
func New(config []config.Upstream) (*Informer, error) {
	if len(config) == 0 {
		return nil, fmt.Errorf("missing configuration")
	}

	for _, c := range config {
		if c.URL == "" {
			return nil, fmt.Errorf("invalid configuration")
		}
	}

	return &Informer{
		config: config,
		client: &http.Client{},
	}, nil
}

// CreateOrUpdate sends the given check configuration to the configured upstreams
func (i *Informer) CreateOrUpdate(check api.Check) error {
	t, n, c, err := check.Config()
	if err != nil {
		return err
	}
	return i.do(context.Background(), http.MethodPost, fmt.Sprintf("checks/%s/%s", t, n), c)
}

// Delete deletes the given check configuration from the configured upstreams
func (i *Informer) Delete(check api.Check) error {
	t, n, _, err := check.Config()
	if err != nil {
		return err
	}
	return i.do(context.Background(), http.MethodDelete, fmt.Sprintf("checks/%s/%s", t, n), "")
}

// DeleteByName removes the given check configuration from the configured upstreams
func (i *Informer) DeleteByName(name string) error {
	return i.do(context.Background(), http.MethodDelete, fmt.Sprintf("checks/%s", name), "")
}

func (i *Informer) do(ctx context.Context, method, endpoint, body string) error {
	for _, c := range i.config {
		url := fmt.Sprintf("%s/%s", c.URL, endpoint)
		req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request for %q: %w", url, err)
		}

		for h, v := range c.Headers {
			req.Header.Add(h, v)
		}

		resp, err := i.client.Do(req)
		if err != nil {
			if b, e := io.ReadAll(req.Body); e == nil {
				err = fmt.Errorf("%w: %s", err, string(b))
			}
			return fmt.Errorf("failed to %s %q: %w", method, url, err)
		}

		defer func() { _ = resp.Body.Close() }()
	}

	return nil
}
