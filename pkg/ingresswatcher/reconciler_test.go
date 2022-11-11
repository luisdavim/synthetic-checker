package ingresswatcher

import (
	"testing"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestPredicates(t *testing.T) {
	tests := []struct {
		name      string
		expected  bool
		ObjectOld client.Object
		ObjectNew client.Object
	}{
		{
			name:     "delete no skip",
			expected: true,
			ObjectOld: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "false",
					},
				},
			},
			ObjectNew: nil,
		},
		{
			name:      "create no skip",
			expected:  true,
			ObjectOld: nil,
			ObjectNew: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "false",
					},
				},
			},
		},
		{
			name:      "create no annotations",
			expected:  true,
			ObjectOld: nil,
			ObjectNew: &netv1.Ingress{},
		},
		{
			name:      "create skip",
			expected:  false,
			ObjectOld: nil,
			ObjectNew: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "true",
					},
				},
			},
		},
		{
			name:     "update skip",
			expected: false,
			ObjectOld: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "true",
					},
				},
			},
			ObjectNew: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "true",
					},
				},
			},
		},
		{
			name:     "update skip change",
			expected: true,
			ObjectOld: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "false",
					},
				},
			},
			ObjectNew: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "true",
					},
				},
			},
		},
		{
			name:     "update no skip change",
			expected: true,
			ObjectOld: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "true",
					},
				},
			},
			ObjectNew: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						skipAnnotation: "false",
					},
				},
			},
		},
		{
			name:      "update no annotations",
			expected:  true,
			ObjectOld: &netv1.Ingress{},
			ObjectNew: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
			},
		},
		{
			name:      "no change",
			expected:  false,
			ObjectOld: &netv1.Ingress{},
			ObjectNew: &netv1.Ingress{},
		},
		{
			name:     "change skip",
			expected: false,
			ObjectOld: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
					Annotations: map[string]string{
						skipAnnotation: "true",
					},
				},
			},
			ObjectNew: &netv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 2,
					Annotations: map[string]string{
						skipAnnotation: "true",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		p := predicates()
		t.Run(tt.name, func(t *testing.T) {
			var actual bool
			switch {
			case tt.ObjectOld == nil:
				actual = p.Create(event.CreateEvent{
					Object: tt.ObjectNew,
				})
			case tt.ObjectNew == nil:
				actual = p.Delete(event.DeleteEvent{
					Object: tt.ObjectOld,
				})
			default:
				actual = p.Update(event.UpdateEvent{
					ObjectOld: tt.ObjectOld,
					ObjectNew: tt.ObjectNew,
				})
			}
			if tt.expected != actual {
				t.Errorf("unexpected result: want %v, got %v", tt.expected, actual)
			}
		})
	}
}
