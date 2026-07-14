package update

import "testing"

func TestNormalizeToolNameAntigravityAliases(t *testing.T) {
	cases := []string{"agy", "antigravity", "gemini", "gemini-cli"}
	for _, in := range cases {
		if got := NormalizeToolName(in); got != "agy" {
			t.Fatalf("NormalizeToolName(%q) = %q, want agy", in, got)
		}
	}
}

func TestAntigravityToolDoesNotMatchAgAlias(t *testing.T) {
	for _, tool := range Tools {
		if tool.BinaryName != "agy" {
			continue
		}
		if tool.MatchesName("ag") {
			t.Fatalf("did not expect agy tool to match ag alias")
		}
		return
	}
	t.Fatal("agy tool not found")
}
