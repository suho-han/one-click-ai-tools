package usage

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/execenv"
)

// commandOutput runs a command with a per-command timeout derived from the
// caller's context, so provider-wide cancellation (e.g. the GetUsage
// deadline) also kills the subprocess.
func commandOutput(parent context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := execenv.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}
