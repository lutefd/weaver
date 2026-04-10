package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainVersionCommand(t *testing.T) {
	if os.Getenv("WEAVER_TEST_MAIN") == "1" {
		os.Args = []string{"weaver", "version"}
		main()
		os.Exit(0)
	}

	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = repo
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}

	testCmd := exec.Command(os.Args[0], "-test.run=TestMainVersionCommand")
	testCmd.Dir = repo
	testCmd.Env = append(os.Environ(), "WEAVER_TEST_MAIN=1")
	output, err := testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main helper failed: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got == "" {
		t.Fatalf("main output = %q, want non-empty version output", got)
	}
	if strings.Contains(string(output), filepath.Base(repo)) {
		t.Fatalf("main output leaked unexpected repo path: %q", output)
	}
}

func TestMainInvalidCommandExitsNonZero(t *testing.T) {
	if os.Getenv("WEAVER_TEST_MAIN_INVALID") == "1" {
		os.Args = []string{"weaver", "not-a-command"}
		main()
		t.Fatal("main() returned, want os.Exit")
	}

	testCmd := exec.Command(os.Args[0], "-test.run=TestMainInvalidCommandExitsNonZero")
	testCmd.Env = append(os.Environ(), "WEAVER_TEST_MAIN_INVALID=1")
	output, err := testCmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("main helper error = %v, want *exec.ExitError\n%s", err, output)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", exitErr.ExitCode(), output)
	}
}
