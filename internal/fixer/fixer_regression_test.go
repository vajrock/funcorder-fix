package fixer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vajrock/funcorder-fix/internal/config"
	"github.com/vajrock/funcorder-fix/internal/fixer"
)

func TestDogfood_OwnSource(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)

	dirs := []string{
		filepath.Join("..", "..", "internal"),
		filepath.Join("..", "..", "cmd"),
	}

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			// Skip test files — they may contain inline Go source strings
			// that intentionally have violations.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			result := f.ProcessFile(path)
			if result.Error != nil {
				t.Errorf("%s: unexpected error: %v", path, result.Error)
				return nil
			}
			if result.Violations != 0 {
				t.Errorf("%s: %d violations in own source code", path, result.Violations)
			}
			if result.Fixed {
				t.Errorf("%s: own source should not need fixing", path)
			}
			return nil
		})
		if err != nil {
			t.Errorf("walk %s: %v", dir, err)
		}
	}
}

func TestDogfood_GoldenFiles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Fix = true

	f := fixer.NewFixer(cfg)

	goldenDirs := []string{
		filepath.Join("..", "..", "testdata", "golden"),
		filepath.Join("..", "..", "examples", "golden"),
	}

	for _, dir := range goldenDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("cannot read %s: %v", dir, err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".go" {
				continue
			}
			path := filepath.Join(dir, e.Name())
			result := f.ProcessFile(path)
			if result.Error != nil {
				t.Errorf("%s: unexpected error: %v", path, result.Error)
				continue
			}
			if result.Violations != 0 {
				t.Errorf("%s: golden file has %d violations", path, result.Violations)
			}
			if result.Fixed {
				t.Errorf("%s: golden file should not need fixing", path)
			}
		}
	}
}
