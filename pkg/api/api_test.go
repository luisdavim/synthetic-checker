package api

import "testing"

func TestEvaluate(t *testing.T) {
	type expected struct {
		allFailed bool
		anyFailed bool
	}
	tests := []struct {
		name     string
		status   Statuses
		expected expected
	}{
		{
			name: "all OK",
			status: Statuses{
				"foo": {
					OK: true,
				},
			},
			expected: expected{
				allFailed: false,
				anyFailed: false,
			},
		},
		{
			name: "all KO",
			status: Statuses{
				"foo": {
					OK: false,
				},
			},
			expected: expected{
				allFailed: true,
				anyFailed: true,
			},
		},
		{
			name: "one failed",
			status: Statuses{
				"foo": {
					OK: true,
				},
				"bar": {
					OK: false,
				},
			},
			expected: expected{
				allFailed: false,
				anyFailed: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allFailed, anyFailed := tt.status.Evaluate()
			if allFailed != tt.expected.allFailed || anyFailed != tt.expected.anyFailed {
				t.Errorf("unexpected result, wanted: %v,%v; got: %v,%v", tt.expected.allFailed, tt.expected.anyFailed, allFailed, anyFailed)
			}
		})
	}
}
