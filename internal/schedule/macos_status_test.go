package schedule

import (
	"errors"
	"os/exec"
	"testing"
)

func TestMacOSStatus_UsesLaunchctlListOutcome(t *testing.T) {
	origList := launchctlList
	t.Cleanup(func() { launchctlList = origList })

	cases := []struct {
		name    string
		loaded  bool
		listErr error
		want    string
		wantErr bool
	}{
		{"loaded", true, nil, "enabled", false},
		{"not loaded", false, nil, "disabled", false},
		{"real failure", false, errors.New("launchd unreachable"), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			launchctlList = func(label string) (bool, error) {
				if label != "com.oct.agent-update" {
					t.Fatalf("launchctlList label = %q, want com.oct.agent-update", label)
				}
				return tc.loaded, tc.listErr
			}

			m := &MacOS{LabelPrefix: "com.oct"}
			got, err := m.Status(AgentUpdateTask)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Status() = %q, nil error; want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Status() error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Status() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The real launchctl path must classify a definitely-absent label as a normal
// "not loaded" (no error) rather than a failure — this is the exact
// misreporting the deferred audit item called out.
func TestRealLaunchctlList_ClassifiesAbsentLabelAsNotLoaded(t *testing.T) {
	if _, err := exec.LookPath("launchctl"); err != nil {
		t.Skip("launchctl not available")
	}

	loaded, err := realLaunchctlList("com.oct.definitely-not-a-real-agent")
	if err != nil {
		t.Fatalf("realLaunchctlList() error: %v, stderr-classified outcome wanted", err)
	}
	if loaded {
		t.Fatal("unexpectedly loaded: com.oct.definitely-not-a-real-agent")
	}
}
