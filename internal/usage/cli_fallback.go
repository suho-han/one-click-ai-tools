package usage

import (
	"bytes"
	"context"
	"fmt"
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
		trimmed := strings.TrimSpace(out.String())
		if trimmed == "" {
			return "", err
		}
		// Keep the command's own diagnostics visible, matching plan.go.
		return "", fmt.Errorf("%w: %s", err, trimmed)
	}

	return strings.TrimSpace(out.String()), nil
}
