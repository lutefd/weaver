package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Result struct {
	Args     []string
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(ctx context.Context, args ...string) (Result, error)
	RepoRoot() string
}

type CLIRunner struct {
	repoRoot string
	stdout   io.Writer
}

func NewCLIRunner(repoRoot string, stdout io.Writer) *CLIRunner {
	return &CLIRunner{
		repoRoot: repoRoot,
		stdout:   stdout,
	}
}

func (r *CLIRunner) Run(ctx context.Context, args ...string) (Result, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", args...)
	if r.repoRoot != "" {
		cmd.Dir = r.repoRoot
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Args:   append([]string(nil), args...),
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if ok := AsExitError(err, &exitErr); ok {
		result.ExitCode = exitErr.ExitCode()
	}

	return result, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

func (r *CLIRunner) RepoRoot() string {
	return r.repoRoot
}

func DiscoverRepoRoot(ctx context.Context, cwd string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func AsExitError(err error, target **exec.ExitError) bool {
	if err == nil {
		return false
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	*target = exitErr
	return true
}
