package fixer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/vajrock/funcorder-fix/internal/config"
)

func FuzzProcessFile(f *testing.F) {
	// Seed with all testdata source files.
	testdataDir := filepath.Join("..", "..", "testdata", "src")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		f.Fatalf("cannot read testdata/src: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(testdataDir, e.Name()))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		// Parse the fuzzed input. If it doesn't parse, skip.
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "fuzz.go", src, parser.ParseComments)
		if err != nil {
			return
		}

		cfg := config.DefaultConfig()
		cfg.Fix = true
		fxr := NewFixer(cfg)

		// Write to temp file for ProcessFile.
		dir := t.TempDir()
		path := filepath.Join(dir, "fuzz.go")
		if err := os.WriteFile(path, src, 0644); err != nil {
			t.Fatal(err)
		}

		result := fxr.ProcessFile(path)
		if result.Error != nil {
			// Errors are acceptable (e.g., partial parse failures).
			return
		}

		output := src
		if result.Fixed {
			output = result.FixedContent
		}

		// Invariant 1: output must parse.
		fset2 := token.NewFileSet()
		file2, err := parser.ParseFile(fset2, "fuzz_out.go", output, parser.ParseComments)
		if err != nil {
			t.Fatalf("fixed output doesn't parse: %v\noutput:\n%s", err, output)
		}

		// Invariant 2: package name preserved.
		if file.Name.Name != file2.Name.Name {
			t.Fatalf("package name changed: %q → %q", file.Name.Name, file2.Name.Name)
		}

		// Invariant 3: top-level declaration names preserved.
		origNames := collectDeclNames(file)
		fixedNames := collectDeclNames(file2)
		if len(origNames) != len(fixedNames) {
			t.Fatalf("decl count changed: %d → %d", len(origNames), len(fixedNames))
		}
		origSet := make(map[string]int)
		for _, n := range origNames {
			origSet[n]++
		}
		for _, n := range fixedNames {
			origSet[n]--
		}
		for name, count := range origSet {
			if count != 0 {
				t.Fatalf("decl %q count changed by %d", name, count)
			}
		}

		// Invariant 4: idempotency — fixing the output should produce 0 new violations.
		if result.Fixed {
			if err := os.WriteFile(path, output, 0644); err != nil {
				t.Fatal(err)
			}
			result2 := fxr.ProcessFile(path)
			if result2.Error != nil {
				t.Fatalf("second pass error: %v", result2.Error)
			}
			if result2.Fixed {
				t.Fatal("second pass still produced fixes — not idempotent")
			}
		}
	})
}

func collectDeclNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			names = append(names, d.Name.Name)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						names = append(names, n.Name)
					}
				}
			}
		}
	}
	return names
}

func FuzzSpliceBytes(f *testing.F) {
	f.Add([]byte("hello world"), 5, 10, []byte("_there_"))
	f.Add([]byte("abcdef"), 0, 6, []byte("XYZ"))
	f.Add([]byte("abcdef"), 3, 3, []byte(""))
	f.Add([]byte(""), 0, 0, []byte("new"))

	f.Fuzz(func(t *testing.T, src []byte, start, end int, replacement []byte) {
		// Normalize start/end to valid range.
		if len(src) == 0 {
			start, end = 0, 0
		} else {
			if start < 0 {
				start = 0
			}
			if end < start {
				end = start
			}
			if start > len(src) {
				start = len(src)
			}
			if end > len(src) {
				end = len(src)
			}
		}

		result := spliceBytes(src, start, end, replacement)

		// Invariant 1: length.
		expectedLen := len(src) - (end - start) + len(replacement)
		if len(result) != expectedLen {
			t.Fatalf("len=%d, expected %d (src=%d, start=%d, end=%d, repl=%d)",
				len(result), expectedLen, len(src), start, end, len(replacement))
		}

		// Invariant 2: prefix preserved.
		for i := 0; i < start; i++ {
			if result[i] != src[i] {
				t.Fatalf("prefix byte %d: got %d, want %d", i, result[i], src[i])
			}
		}

		// Invariant 3: suffix preserved.
		for i := 0; i < len(src)-end; i++ {
			resultIdx := start + len(replacement) + i
			srcIdx := end + i
			if result[resultIdx] != src[srcIdx] {
				t.Fatalf("suffix byte %d: got %d, want %d", i, result[resultIdx], src[srcIdx])
			}
		}
	})
}
