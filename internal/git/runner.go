package git

import (
	"bytes"
	"context"
	"errors"
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

	if r.stdout != nil && isMutating(args) {
		fmt.Fprintf(r.stdout, "+ git %s\n", strings.Join(args, " "))
	}

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
		result.ExitCode = 0
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
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

func isMutating(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "checkout", "switch", "rebase", "merge", "cherry-pick", "commit", "reset":
		return true
	case "branch":
		for _, arg := range args[1:] {
			if arg == "-f" || arg == "--force" {
				return true
			}
		}
		return false
	default:
		return false
	}
}
