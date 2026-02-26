package fixer_test

import (
	"os"
	"path/filepath"
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
