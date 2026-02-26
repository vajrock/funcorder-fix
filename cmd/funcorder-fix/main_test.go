package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	dir, err := os.MkdirTemp("", "funcorder-fix-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	binaryPath = filepath.Join(dir, "funcorder-fix")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

func testdataPath(parts ...string) string {
	base := filepath.Join("..", "..", "testdata")
	return filepath.Join(append([]string{base}, parts...)...)
}

func runBinary(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("failed to run binary: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func TestCLI_DetectOnly(t *testing.T) {
	stdout, stderr, _ := runBinary(t, testdataPath("src", "exported_only.go"))

	if stdout != "" {
		t.Errorf("expected empty stdout in detect mode, got %q", stdout)
	}
	if !strings.Contains(stderr, "violations") {
		t.Errorf("expected stderr to mention violations, got %q", stderr)
	}
}

func TestCLI_FixStdout(t *testing.T) {
	stdout, _, _ := runBinary(t, "--fix", testdataPath("src", "exported_only.go"))

	if !strings.Contains(stdout, "package testpkg") {
		t.Error("expected stdout to contain valid Go code")
	}
	if !strings.Contains(stdout, "func (w *Worker) Start()") {
		t.Error("expected stdout to contain reordered methods")
	}
}

func TestCLI_FixWrite(t *testing.T) {
	// Copy source file to temp dir.
	dir := t.TempDir()
	src, err := os.ReadFile(testdataPath("src", "exported_only.go"))
	if err != nil {
		t.Fatal(err)
	}
	tmpFile := filepath.Join(dir, "exported_only.go")
	if err := os.WriteFile(tmpFile, src, 0644); err != nil {
		t.Fatal(err)
	}

	_, _, exitCode := runBinary(t, "--fix", "-w", tmpFile)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	written, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "func (w *Worker) Start()") {
		t.Error("expected file to be rewritten with fixed content")
	}
}

func TestCLI_FixDiff(t *testing.T) {
	stdout, _, _ := runBinary(t, "--fix", "-d", testdataPath("src", "exported_only.go"))

	if !strings.Contains(stdout, "---") || !strings.Contains(stdout, "+++") {
		t.Errorf("expected diff markers in stdout, got %q", stdout)
	}
}

func TestCLI_ListMode(t *testing.T) {
	stdout, _, _ := runBinary(t, "-l", testdataPath("src", "exported_only.go"))

	if !strings.Contains(stdout, "exported_only.go") {
		t.Errorf("expected stdout to list file path, got %q", stdout)
	}
}

func TestCLI_VerboseMode(t *testing.T) {
	_, stderr, _ := runBinary(t, "-v", testdataPath("src", "exported_only.go"))

	if !strings.Contains(stderr, "violations") {
		t.Errorf("expected verbose stderr to mention violations, got %q", stderr)
	}
}

func TestCLI_NoConstructor(t *testing.T) {
	_, stderr, _ := runBinary(t, "-v", "--no-constructor", testdataPath("src", "constructor_only.go"))

	if strings.Contains(stderr, "1 violations") || strings.Contains(stderr, "2 violations") {
		t.Errorf("expected 0 violations with --no-constructor, got stderr: %q", stderr)
	}
}

func TestCLI_NoExported(t *testing.T) {
	_, stderr, _ := runBinary(t, "-v", "--no-exported", testdataPath("src", "exported_only.go"))

	if strings.Contains(stderr, "1 violations") || strings.Contains(stderr, "2 violations") {
		t.Errorf("expected 0 violations with --no-exported, got stderr: %q", stderr)
	}
}

func TestCLI_NoArgs(t *testing.T) {
	// Running without args processes "." — should not panic.
	cmd := exec.Command(binaryPath)
	cmd.Dir = t.TempDir()
	err := cmd.Run()
	// It might exit 0 (no Go files) or 1 — just check no panic.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			_ = exitErr // any exit code is fine, just no crash
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestCLI_NonExistentPath(t *testing.T) {
	_, stderr, exitCode := runBinary(t, "/nonexistent/path")

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if stderr == "" {
		t.Error("expected stderr error message")
	}
}

func TestCLI_WildcardDotDotDot(t *testing.T) {
	// Create a temp dir with a subdirectory containing Go files.
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	src := "package sub\ntype S struct{}\nfunc (s *S) b() {}\nfunc (s *S) A() {}\n"
	if err := os.WriteFile(filepath.Join(subdir, "test.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr, _ := runBinary(t, dir+"/...")

	if !strings.Contains(stderr, "violations") {
		t.Errorf("expected violations via recursive wildcard, got stderr: %q", stderr)
	}
}
