package model

import "testing"

func TestHasSubstantialToolParameters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		params map[string]any
		want   bool
	}{
		{name: "nil", params: nil, want: false},
		{name: "empty", params: map[string]any{}, want: false},
		{name: "type only", params: map[string]any{"type": "object"}, want: false},
		{name: "empty properties", params: map[string]any{"type": "object", "properties": map[string]any{}}, want: false},
		{name: "with properties", params: map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}}, want: true},
		{name: "required only", params: map[string]any{"type": "object", "required": []string{"x"}}, want: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := HasSubstantialToolParameters(tc.params); got != tc.want {
				t.Fatalf("HasSubstantialToolParameters(%v)=%v, want %v", tc.params, got, tc.want)
			}
		})
	}
}
