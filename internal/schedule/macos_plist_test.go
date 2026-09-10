package schedule

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// TestRenderLaunchAgentPlistEscapesSpecialCharacters pins XML safety: paths
// containing & < > ' " must round-trip through an XML parser unchanged,
// because BinaryPath/LogPath go straight into <string> elements.
func TestRenderLaunchAgentPlistEscapesSpecialCharacters(t *testing.T) {
	weirdBin := `/opt/weird & tools <v1>/oct`
	weirdLog := `/Users/dev/logs & <oct>.log`
	data := launchAgentTemplateData{
		Label:      "com.oct.test",
		BinaryPath: weirdBin,
		Command:    "usage --json",
		Interval:   "daily",
		Hour:       9,
		LogPath:    weirdLog,
	}

	var buf bytes.Buffer
	if err := renderLaunchAgentPlist(&buf, data); err != nil {
		t.Fatalf("renderLaunchAgentPlist() error = %v", err)
	}
	rendered := buf.String()
	if !strings.Contains(rendered, "com.oct.test") {
		t.Fatalf("label missing from rendered plist:\n%s", rendered)
	}

	// Collect every <string> element's decoded text via a token walk (the
	// elements are nested, so a flat struct unmarshal would see nothing).
	dec := xml.NewDecoder(strings.NewReader(rendered))
	var texts []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("rendered plist is not valid XML: %v\n%s", err, rendered)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "string" {
			var s string
			if err := dec.DecodeElement(&s, &se); err != nil {
				t.Fatalf("decode string element failed: %v", err)
			}
			texts = append(texts, s)
		}
	}

	found := map[string]bool{}
	for _, s := range texts {
		found[s] = true
	}
	if !found[weirdBin] {
		t.Fatalf("BinaryPath not round-tripped: %q not in %v", weirdBin, texts)
	}
	if !found[weirdLog] {
		t.Fatalf("LogPath not round-tripped: %q not in %v", weirdLog, texts)
	}
}
