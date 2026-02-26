package fixer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vajrock/funcorder-fix/internal/config"
	"github.com/vajrock/funcorder-fix/internal/fixer"
)

// testdataPath joins "../../testdata" with the given path parts.
func testdataPath(parts ...string) string {
	base := filepath.Join("..", "..", "testdata")
	return filepath.Join(append([]string{base}, parts...)...)
}

func TestProcessFile_NoViolations(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", "no_violations.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations, got %d", result.Violations)
	}
	if result.Fixed {
		t.Error("expected Fixed==false, got true")
	}
	if result.FixedContent != nil {
		t.Errorf("expected FixedContent==nil, got %d bytes", len(result.FixedContent))
	}
}

func TestProcessFile_ConstructorOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", "constructor_only.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations <= 0 {
		t.Errorf("expected >0 violations, got %d", result.Violations)
	}
	if !result.Fixed {
		t.Error("expected Fixed==true, got false")
	}

	golden, err := os.ReadFile(testdataPath("golden", "constructor_only.go"))
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if string(result.FixedContent) != string(golden) {
		t.Errorf("FixedContent does not match golden file.\ngot:\n%s\nwant:\n%s",
			result.FixedContent, golden)
	}
}

func TestProcessFile_ExportedOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", "exported_only.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations <= 0 {
		t.Errorf("expected >0 violations, got %d", result.Violations)
	}
	if !result.Fixed {
		t.Error("expected Fixed==true, got false")
	}

	golden, err := os.ReadFile(testdataPath("golden", "exported_only.go"))
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if string(result.FixedContent) != string(golden) {
		t.Errorf("FixedContent does not match golden file.\ngot:\n%s\nwant:\n%s",
			result.FixedContent, golden)
	}
}

func TestProcessFile_MixedViolations(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", "mixed_violations.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations <= 0 {
		t.Errorf("expected >0 violations, got %d", result.Violations)
	}
	if !result.Fixed {
		t.Error("expected Fixed==true, got false")
	}

	golden, err := os.ReadFile(testdataPath("golden", "mixed_violations.go"))
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}
	if string(result.FixedContent) != string(golden) {
		t.Errorf("FixedContent does not match golden file.\ngot:\n%s\nwant:\n%s",
			result.FixedContent, golden)
	}
}

func TestProcessFile_NoFixMode(t *testing.T) {
	cfg := config.DefaultConfig()
	// Fix defaults to false in DefaultConfig

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", "exported_only.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations <= 0 {
		t.Errorf("expected >0 violations, got %d", result.Violations)
	}
	if result.Fixed {
		t.Error("expected Fixed==false, got true")
	}
	if result.FixedContent != nil {
		t.Errorf("expected FixedContent==nil, got %d bytes", len(result.FixedContent))
	}
}

func TestProcessFile_NoConstructorCheck(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true
	cfg.CheckConstructor = false
	cfg.CheckExported = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", "constructor_only.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations with constructor check disabled, got %d", result.Violations)
	}
	if result.Fixed {
		t.Error("expected Fixed==false, got true")
	}
}

func TestProcessFile_NoExportedCheck(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true
	cfg.CheckConstructor = true
	cfg.CheckExported = false

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", "exported_only.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations with exported check disabled, got %d", result.Violations)
	}
	if result.Fixed {
		t.Error("expected Fixed==false, got true")
	}
}

func TestProcessFile_Idempotent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("golden", "mixed_violations.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations on already-fixed file, got %d", result.Violations)
	}
	if result.Fixed {
		t.Error("expected Fixed==false on already-fixed file, got true")
	}
}

// goldenTest is a reusable helper for golden-file comparison tests.
func goldenTest(t *testing.T, name string, expectViolations bool) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("src", name))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	golden, err := os.ReadFile(testdataPath("golden", name))
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	if expectViolations {
		if result.Violations <= 0 {
			t.Errorf("expected >0 violations, got %d", result.Violations)
		}
		if !result.Fixed {
			t.Error("expected Fixed==true, got false")
		}
		got := strings.TrimRight(string(result.FixedContent), "\n")
		want := strings.TrimRight(string(golden), "\n")
		if got != want {
			t.Errorf("FixedContent does not match golden file.\ngot:\n%s\nwant:\n%s",
				got, want)
		}
	} else {
		if result.Violations != 0 {
			t.Errorf("expected 0 violations, got %d", result.Violations)
		}
		if result.Fixed {
			t.Error("expected Fixed==false, got true")
		}
	}
}

func TestProcessFile_WithComments(t *testing.T) {
	goldenTest(t, "with_comments.go", true)
}

func TestProcessFile_WithSpacing(t *testing.T) {
	goldenTest(t, "with_spacing.go", true)
}

func TestProcessFile_MultiStruct(t *testing.T) {
	goldenTest(t, "multi_struct.go", true)
}

func TestProcessFile_MultilineFuncs(t *testing.T) {
	goldenTest(t, "multiline_funcs.go", true)
}

func TestProcessFile_Generics(t *testing.T) {
	goldenTest(t, "generics.go", true)
}

func TestProcessFile_GapFunctions(t *testing.T) {
	goldenTest(t, "gap_functions.go", true)
}

func TestProcessFile_WithComments_Idempotent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("golden", "with_comments.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations on golden file, got %d", result.Violations)
	}
	if result.Fixed {
		t.Error("expected Fixed==false on golden file")
	}
}

func TestProcessFile_MultiStruct_Idempotent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)
	result := f.ProcessFile(testdataPath("golden", "multi_struct.go"))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations on golden file, got %d", result.Violations)
	}
	if result.Fixed {
		t.Error("expected Fixed==false on golden file")
	}
}
