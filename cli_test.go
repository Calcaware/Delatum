package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildCLI compiles the command line tool so the tests can check its output
// and process exit status. The VM tests call the same code in process, where
// os.Exit cannot be observed.
func buildCLI(t *testing.T) string {
	t.Helper()

	name := "delatum"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)

	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build delatum: %v\n%s", err, out)
	}
	return bin
}

// runCLI returns the output of the command and its process exit code.
func runCLI(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode()
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func TestHelloWorldCLI(t *testing.T) {
	bin := buildCLI(t)

	stdout, stderr, code := runCLI(t, bin, "run", "hello.tw")
	if code != 0 {
		t.Fatalf("exit code is %d, want 0; stderr: %s", code, stderr)
	}
	if got, want := stdout, "Hello, world!\n"; got != want {
		t.Fatalf("output is %q, want %q", got, want)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestCLIExitCodeFromHalt(t *testing.T) {
	bin := buildCLI(t)

	path := filepath.Join(t.TempDir(), "exit.tw")
	if err := os.WriteFile(path, []byte("halt 7\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runCLI(t, bin, "run", path)
	if code != 7 {
		t.Fatalf("exit code is %d, want 7; stderr: %s", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}
