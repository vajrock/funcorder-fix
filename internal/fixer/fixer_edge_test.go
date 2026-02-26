package fixer

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vajrock/funcorder-fix/internal/config"
)

// --- spliceBytes tests ---

func TestSpliceBytes_EmptySrc(t *testing.T) {
	result := spliceBytes([]byte{}, 0, 0, []byte("hello"))
	if string(result) != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestSpliceBytes_ZeroRange(t *testing.T) {
	src := []byte("abcdef")
	result := spliceBytes(src, 3, 3, []byte("XY"))
	if string(result) != "abcXYdef" {
		t.Errorf("got %q, want %q", result, "abcXYdef")
	}
}

func TestSpliceBytes_FullReplace(t *testing.T) {
	src := []byte("abcdef")
	result := spliceBytes(src, 0, 6, []byte("XYZ"))
	if string(result) != "XYZ" {
		t.Errorf("got %q, want %q", result, "XYZ")
	}
}

func TestSpliceBytes_EmptyReplacement(t *testing.T) {
	src := []byte("abcdef")
	result := spliceBytes(src, 2, 4, []byte{})
	if string(result) != "abef" {
		t.Errorf("got %q, want %q", result, "abef")
	}
}

func TestSpliceBytes_LengthInvariant(t *testing.T) {
	src := []byte("hello world")
	start, end := 5, 10
	replacement := []byte("_there_")
	result := spliceBytes(src, start, end, replacement)

	expectedLen := len(src) - (end - start) + len(replacement)
	if len(result) != expectedLen {
		t.Errorf("len(result)=%d, expected %d", len(result), expectedLen)
	}
}

func TestSpliceBytes_PreservesPrefix(t *testing.T) {
	src := []byte("abcdef")
	start := 3
	result := spliceBytes(src, start, 5, []byte("XY"))
	if !bytes.Equal(result[:start], src[:start]) {
		t.Errorf("prefix not preserved: got %q, want %q", result[:start], src[:start])
	}
}

func TestSpliceBytes_PreservesSuffix(t *testing.T) {
	src := []byte("abcdef")
	end := 4
	replacement := []byte("XY")
	result := spliceBytes(src, 2, end, replacement)
	suffixStart := 2 + len(replacement)
	if !bytes.Equal(result[suffixStart:], src[end:]) {
		t.Errorf("suffix not preserved: got %q, want %q", result[suffixStart:], src[end:])
	}
}

// --- GetMethodBlock tests ---

func TestGetMethodBlock_WithDocComment(t *testing.T) {
	src := `package foo

// Run starts the process.
func (s *Svc) Run() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	fn := file.Decls[0].(*ast.FuncDecl)
	cp := NewCommentPreserver(fset, file)
	block := cp.GetMethodBlock(fn, []byte(src))

	if block.Name != "Run" {
		t.Errorf("Name = %q, want %q", block.Name, "Run")
	}
	if block.FuncDecl != fn {
		t.Error("FuncDecl mismatch")
	}
	// Doc comment should extend start backwards.
	if block.StartPos >= fn.Pos() {
		t.Error("expected StartPos to be before fn.Pos() due to doc comment")
	}
	if !bytes.Contains([]byte(block.RawText), []byte("// Run starts the process.")) {
		t.Errorf("RawText should contain doc comment, got %q", block.RawText)
	}
}

func TestGetMethodBlock_WithoutDocComment(t *testing.T) {
	src := `package foo

func (s *Svc) Run() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	fn := file.Decls[0].(*ast.FuncDecl)
	cp := NewCommentPreserver(fset, file)
	block := cp.GetMethodBlock(fn, []byte(src))

	if block.StartPos != fn.Pos() {
		t.Errorf("without doc comment, StartPos should equal fn.Pos()")
	}
}

func TestGetMethodBlock_CommentMapVsDoc(t *testing.T) {
	// This tests the critical invariant: inline body comments from function A
	// should NOT be captured as leading comments of function B.
	src := `package foo

func (s *Svc) Run() {
	// some body comment
}

func (s *Svc) Stop() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	// Stop is the second FuncDecl.
	stopFn := file.Decls[1].(*ast.FuncDecl)
	cp := NewCommentPreserver(fset, file)
	block := cp.GetMethodBlock(stopFn, []byte(src))

	// The body comment from Run should NOT appear in Stop's RawText.
	if bytes.Contains([]byte(block.RawText), []byte("some body comment")) {
		t.Error("Stop's RawText should not contain Run's body comment")
	}
}

// --- ProcessFile error cases ---

func TestProcessFile_NonExistentFile(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := NewFixer(cfg)
	result := f.ProcessFile("/nonexistent/path/to/file.go")

	if result.Error == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestProcessFile_SyntaxError(t *testing.T) {
	// Create a temp file with invalid Go syntax.
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(path, []byte("package foo\nfunc {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := NewFixer(cfg)
	result := f.ProcessFile(path)

	if result.Error == nil {
		t.Error("expected parse error for file with syntax errors")
	}
}

func TestProcessFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.go")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := NewFixer(cfg)
	result := f.ProcessFile(path)

	// Empty file should fail to parse (no package clause).
	if result.Error == nil {
		t.Error("expected error for empty file")
	}
}

// --- WriteResult tests ---

func TestWriteResult_NotFixed(t *testing.T) {
	cfg := config.DefaultConfig()
	f := NewFixer(cfg)

	result := &Result{Fixed: false}
	err := f.WriteResult(result)
	if err != nil {
		t.Errorf("WriteResult on not-fixed result should return nil, got %v", err)
	}
}

func TestWriteResult_WriteMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.go")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Write = true
	f := NewFixer(cfg)

	result := &Result{
		FilePath:     path,
		Fixed:        true,
		FixedContent: []byte("fixed content"),
	}
	err := f.WriteResult(result)
	if err != nil {
		t.Fatalf("WriteResult returned error: %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "fixed content" {
		t.Errorf("written = %q, want %q", written, "fixed content")
	}
}

// --- ProcessDirectory tests ---

func TestProcessDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory(dir)

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty dir, got %d", len(results))
	}
}

func TestProcessDirectory_SkipsVendor(t *testing.T) {
	dir := t.TempDir()
	vendorDir := filepath.Join(dir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	vendorFile := filepath.Join(vendorDir, "lib.go")
	if err := os.WriteFile(vendorFile, []byte("package lib\nfunc Foo() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory(dir)

	for _, r := range results {
		if r.FilePath == vendorFile {
			t.Error("vendor/ should be skipped")
		}
	}
}

func TestProcessDirectory_SkipsHidden(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	hiddenFile := filepath.Join(hiddenDir, "secret.go")
	if err := os.WriteFile(hiddenFile, []byte("package secret\nfunc Foo() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory(dir)

	for _, r := range results {
		if r.FilePath == hiddenFile {
			t.Error(".hidden/ should be skipped")
		}
	}
}

func TestProcessDirectory_NonExistent(t *testing.T) {
	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory("/nonexistent/dir")

	if len(results) == 0 {
		t.Fatal("expected at least one result with error")
	}

	hasError := false
	for _, r := range results {
		if r.Error != nil {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("expected error for non-existent directory")
	}
}

func TestProcessDirectory_DotPath(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	src := "package main\ntype S struct{}\nfunc (s *S) b() {}\nfunc (s *S) A() {}\n"
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// Save and restore working directory.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory(".")

	if len(results) == 0 {
		t.Fatal("ProcessDirectory(\".\") returned 0 results, expected at least 1")
	}
}

func TestProcessDirectory_DotDotPath(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	src := "package main\ntype S struct{}\nfunc (s *S) b() {}\nfunc (s *S) A() {}\n"
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory and chdir into it, then walk "..".
	subDir := filepath.Join(dir, "child")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory("..")

	if len(results) == 0 {
		t.Fatal("ProcessDirectory(\"..\") returned 0 results, expected at least 1")
	}
}

func TestProcessDirectory_DotPathStillSkipsHidden(t *testing.T) {
	dir := t.TempDir()

	// Create a hidden directory with a Go file.
	hiddenDir := filepath.Join(dir, ".secret")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "hidden.go"), []byte("package secret\nfunc Foo() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a visible Go file so we know walking works.
	if err := os.WriteFile(filepath.Join(dir, "visible.go"), []byte("package main\nfunc Bar() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory(".")

	for _, r := range results {
		if strings.Contains(r.FilePath, ".secret") {
			t.Error(".secret/ directory should still be skipped when walking \".\"")
		}
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for visible.go")
	}
}

func TestProcessDirectory_ProcessesGoFiles(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	src := "package main\ntype S struct{}\nfunc (s *S) b() {}\nfunc (s *S) A() {}\n"
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	// Non-Go file should be ignored.
	txtFile := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	f := NewFixer(cfg)
	results := f.ProcessDirectory(dir)

	if len(results) != 1 {
		t.Errorf("expected 1 result (only .go file), got %d", len(results))
	}
	if len(results) > 0 && results[0].FilePath != goFile {
		t.Errorf("expected result for %s, got %s", goFile, results[0].FilePath)
	}
}
