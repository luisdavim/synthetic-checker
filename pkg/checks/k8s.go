package checks

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	konfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/luisdavim/synthetic-checker/pkg/api"
	"github.com/luisdavim/synthetic-checker/pkg/config"
)

type k8sCheck struct {
	config *config.K8sCheck
	client client.Client
}

var k8sClient *client.Client

func NewK8sCheck(name string, config config.K8sCheck) (api.Check, error) {
	if name == "" {
		return nil, fmt.Errorf("CheckName must not be empty")
	}
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = time.Second
	}
	if config.Name == "" {
		return nil, fmt.Errorf("resource name must not be empty")
	}
	if config.Namespace == "" {
		config.Namespace = "default"
	}

	if k8sClient == nil {
		if c, err := client.New(konfig.GetConfigOrDie(), client.Options{}); err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		} else {
			k8sClient = &c
		}
	}

	return &k8sCheck{
		config: &config,
		client: *k8sClient,
	}, nil
}

// Interval indicates how often the check should be performed
func (c *k8sCheck) Interval() time.Duration {
	return c.config.Interval
}

// Interval indicates how often the check should be performed
func (c *k8sCheck) Execute(ctx context.Context) (bool, error) {
	u := &unstructured.Unstructured{}
	gvk, _ := schema.ParseKindArg(c.config.Kind)
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Kind:    gvk.Kind,
		Version: gvk.Version,
	})
	if err := c.client.Get(context.Background(), client.ObjectKey{
		Namespace: c.config.Namespace,
		Name:      c.config.Name,
	}, u); err != nil {
		return false, fmt.Errorf("failed to get: %w", err)
	}

	res, err := status.Compute(u)
	if err != nil {
		return false, err
	}
	ok := res.Status == status.CurrentStatus
	if !ok {
		err = fmt.Errorf("wrong resource state: %s - %s", res.Status, res.Message)
	}
	return ok, err
}
