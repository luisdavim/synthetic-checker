package checks

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/luisdavim/synthetic-checker/pkg/config"
)

func y2u(t *testing.T, spec string) *unstructured.Unstructured {
	j, err := yaml.YAMLToJSON([]byte(spec))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	u, _, err := unstructured.UnstructuredJSONScheme.Decode(j, nil, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	return u.(*unstructured.Unstructured)
}

func TestK8sCheck(t *testing.T) {
	type expected struct {
		ok  bool
		err error
	}
	tests := []struct {
		name     string
		config   config.K8sCheck
		object   string
		expected expected
	}{
		{
			name: "failed deployment",
			expected: expected{
				ok:  false,
				err: errors.New("1 of 1 resources are not ok: test: wrong resource state: InProgress - Deployment not Available"),
			},
			config: config.K8sCheck{
				Kind:      "Deployment.v1.apps",
				Name:      "test",
				Namespace: "test",
			},
			object: `
apiVersion: apps/v1
kind: Deployment
metadata:
   name: test
   generation: 1
   namespace: test
status:
   observedGeneration: 1
   updatedReplicas: 1
   readyReplicas: 1
   availableReplicas: 1
   replicas: 1
   observedGeneration: 1
   conditions:
    - type: Progressing
      status: "True"
      reason: NewReplicaSetAvailable
    - type: Available
      status: "False"
`,
		},
		{
			name: "deployment OK",
			expected: expected{
				ok:  true,
				err: nil,
			},
			config: config.K8sCheck{
				Kind:      "Deployment.v1.apps",
				Name:      "test",
				Namespace: "test",
			},
			object: `
apiVersion: apps/v1
kind: Deployment
metadata:
   name: test
   generation: 1
   namespace: test
status:
   observedGeneration: 1
   updatedReplicas: 1
   readyReplicas: 1
   availableReplicas: 1
   replicas: 1
   conditions:
    - type: Progressing
      status: "True"
      reason: NewReplicaSetAvailable
    - type: Available
      status: "True"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient = fake.NewClientBuilder().WithObjects(y2u(t, tt.object)).Build()
			c, err := NewK8sCheck("test", tt.config)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			ok, err := c.Execute(context.TODO())
			if err != nil && tt.expected.err == nil {
				t.Errorf("unexpected error: %v", err)
			}
			if (err != nil && tt.expected.err != nil) && (err.Error() != tt.expected.err.Error()) {
				t.Errorf("unexpected error, wanted: %v, got: %v", tt.expected.err, err)
			}
			if ok != tt.expected.ok {
				t.Errorf("unexpected status, wanted: %t, got: %t", tt.expected.ok, ok)
			}
		})
	}
}
