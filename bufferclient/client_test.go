package bufferclient

import (
	"encoding/json"
	"testing"
)

func ptr(s string) *string { return &s }

func TestEditPostInputTextMarshal(t *testing.T) {
	cases := []struct {
		name       string
		input      EditPostInput
		wantHasKey bool
		wantValue  string
	}{
		{
			name:       "nil text omits the text key",
			input:      EditPostInput{ID: "x", Text: nil},
			wantHasKey: false,
		},
		{
			name:       "empty string pointer keeps an empty text key",
			input:      EditPostInput{ID: "x", Text: ptr("")},
			wantHasKey: true,
			wantValue:  "",
		},
		{
			name:       "non-empty string pointer keeps the text value",
			input:      EditPostInput{ID: "x", Text: ptr("hello")},
			wantHasKey: true,
			wantValue:  "hello",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatalf("Marshal returned error: %v", err)
			}
			var raw map[string]interface{}
			if err := json.Unmarshal(body, &raw); err != nil {
				t.Fatalf("Unmarshal into map failed: %v", err)
			}
			value, hasKey := raw["text"]
			if hasKey != tc.wantHasKey {
				t.Fatalf("text key presence = %v, want %v (json: %s)", hasKey, tc.wantHasKey, body)
			}
			if tc.wantHasKey && value != tc.wantValue {
				t.Fatalf("text value = %q, want %q (json: %s)", value, tc.wantValue, body)
			}
		})
	}
}
