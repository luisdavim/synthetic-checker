package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

var _ api.Check = &k8sPinger{}

type k8sPinger struct {
	name   string
	config *config.K8sPing
	client client.Reader
	dialer *net.Dialer
}

func NewK8sPing(name string, config config.K8sPing) (api.Check, error) {
	if name == "" {
		return nil, fmt.Errorf("CheckName must not be empty")
	}
	if config.Protocol == "" {
		config.Protocol = "tcp"
	}
	if config.Interval.Duration == 0 {
		config.Interval = metav1.Duration{Duration: 30 * time.Second}
	}
	if config.Timeout.Duration == 0 {
		config.Timeout = metav1.Duration{Duration: time.Second}
	}

	if k8sClient == nil {
		kfg, err := konfig.GetConfig()
		if err != nil {
			return nil, err
		}
		if c, err := client.New(kfg, client.Options{}); err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		} else {
			k8sClient = c
		}
	}

	return &k8sPinger{
		name:   name,
		config: &config,
		client: k8sClient,
	}, nil
}

func (c *k8sPinger) Equal(other *k8sPinger) bool {
	return c.config.Equal(*other.config)
}

func (c *k8sPinger) Config() (string, string, string, error) {
	b, err := json.Marshal(c.config)
	if err != nil {
		return "", "", "", err
	}
	return "k8sping", c.name, string(b), nil
}

// Interval indicates how often the check should be performed
func (c *k8sPinger) Interval() metav1.Duration {
	return c.config.Interval
}

// InitialDelay indicates how long to delay the check start
func (c *k8sPinger) InitialDelay() metav1.Duration {
	return c.config.InitialDelay
}

func (c *k8sPinger) Execute(ctx context.Context) (bool, error) {
	if c.dialer == nil {
		c.dialer = &net.Dialer{
			Timeout: c.config.Timeout.Duration,
		}
	}

	pl, err := c.do(ctx)
	if err != nil {
		return false, err
	}

	resCount := len(pl.Items)
	if resCount == 0 {
		return false, fmt.Errorf("no resources found")
	}

	allOK := true
	var errs []error

	for _, p := range pl.Items {
		address := fmt.Sprintf("%s.%s:%d", p.Name, p.Namespace, c.config.Port)
		conn, err := c.dialer.DialContext(ctx, c.config.Protocol, address)
		if err != nil {
			allOK = false
			errs = append(errs, err)
		}
		if conn != nil {
			_ = conn.Close()
		}
	}

	errCount := len(errs)
	for _, e := range errs {
		err = fmt.Errorf("%d of %d resources are not reachable: %w", errCount, resCount, e)
	}
	return allOK, err
}

func (c *k8sPinger) do(ctx context.Context) (*corev1.PodList, error) {
	pl := &corev1.PodList{}
	opts := &client.ListOptions{
		Namespace: c.config.Namespace,
	}
	if c.config.LabelSelector != "" {
		var err error
		opts.LabelSelector, err = labels.Parse(c.config.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}
	}
	if err := c.client.List(ctx, pl, opts); err != nil {
		return nil, fmt.Errorf("failed to list: %w", err)
	}

	return pl, nil
}
