package update

import (
	"context"
	"os/exec"

	"github.com/suho-han/one-click-tools/internal/execenv"
)

func commandEnv() []string {
	return execenv.Environ()
}

func withPathEnv(env []string, pathValue string) []string {
	return execenv.WithPathEnv(env, pathValue)
}

func bootstrapPATH(base string) string {
	return execenv.BuildPATH(base)
}

func commandWithEnv(name string, args ...string) *exec.Cmd {
	return execenv.Command(name, args...)
}

func commandContextWithEnv(ctx context.Context, name string, args ...string) *exec.Cmd {
	return execenv.CommandContext(ctx, name, args...)
}

func resolveExecutable(name string) string {
	return execenv.ResolveExecutable(name)
}

func lookPathWithBootstrap(name string) (string, error) {
	return execenv.LookPath(name)
}
