package cmd

import "testing"

func TestShouldAutoJSONFallback(t *testing.T) {
	tests := []struct {
		name     string
		jsonMode bool
		isTTY    bool
		want     bool
	}{
		{name: "json already requested", jsonMode: true, isTTY: false, want: false},
		{name: "tty and no json flag", jsonMode: false, isTTY: true, want: false},
		{name: "non tty and no json flag", jsonMode: false, isTTY: false, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAutoJSONFallback(tc.jsonMode, tc.isTTY)
			if got != tc.want {
				t.Fatalf("shouldAutoJSONFallback(%v, %v) = %v, want %v", tc.jsonMode, tc.isTTY, got, tc.want)
			}
		})
	}
}
