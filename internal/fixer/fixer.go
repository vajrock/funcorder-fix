package fixer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/vajrock/funcorder-fix/internal/config"
	"github.com/vajrock/funcorder-fix/internal/detector"
)

// Fixer orchestrates detection and fixing of funcorder violations.
type Fixer struct {
	config *config.Config
}

// NewFixer creates a new Fixer with the given configuration.
func NewFixer(cfg *config.Config) *Fixer {
	return &Fixer{config: cfg}
}

// Result contains the result of fixing a file.
type Result struct {
	// FilePath is the path to the processed file.
	FilePath string

	// Violations is the number of violations found.
	Violations int

	// Fixed indicates if the file was fixed.
	Fixed bool

	// OriginalContent is the original file content.
	OriginalContent []byte

	// FixedContent is the fixed file content.
	FixedContent []byte

	// Error is any error that occurred during processing.
	Error error
}

// ProcessFile processes a single file for funcorder violations.
func (f *Fixer) ProcessFile(filePath string) *Result {
	result := &Result{
		FilePath: filePath,
	}

	// Read the file
	src, err := os.ReadFile(filePath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read file: %w", err)
		return result
	}
	result.OriginalContent = src

	// Parse the file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments|parser.AllErrors)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse file: %w", err)
		return result
	}

	// Detect violations
	det := detector.NewDetector(fset, f.config)
	report := det.Detect(file, filePath)
	result.Violations = len(report.Violations)

	// If no violations or not in fix mode, return
	if !report.HasViolations() || !f.config.Fix {
		return result
	}

	// Fix the file
	fixedContent, err := f.fixFile(fset, file, src, report)
	if err != nil {
		result.Error = fmt.Errorf("failed to fix file: %w", err)
		return result
	}

	result.FixedContent = fixedContent
	result.Fixed = true

	return result
}

// ProcessDirectory processes all Go files in a directory.
func (f *Fixer) ProcessDirectory(dirPath string) []*Result {
	var results []*Result

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and hidden directories
		if info.IsDir() {
			if info.Name() == "vendor" || (len(info.Name()) > 0 && info.Name()[0] == '.') {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files
		if filepath.Ext(path) != ".go" {
			return nil
		}

		result := f.ProcessFile(path)
		results = append(results, result)
		return nil
	})

	if err != nil {
		results = append(results, &Result{
			FilePath: dirPath,
			Error:    fmt.Errorf("failed to walk directory: %w", err),
		})
	}

	return results
}

// WriteResult writes the fixed content to the file or displays a diff.
func (f *Fixer) WriteResult(result *Result) error {
	if !result.Fixed || len(result.FixedContent) == 0 {
		return nil
	}

	if f.config.Write {
		return os.WriteFile(result.FilePath, result.FixedContent, 0644)
	}

	if f.config.Diff {
		// Print diff
		fmt.Printf("--- %s\n", result.FilePath)
		fmt.Printf("+++ %s (fixed)\n", result.FilePath)
		fmt.Println(string(result.FixedContent))
		return nil
	}

	// Just print to stdout
	fmt.Println(string(result.FixedContent))
	return nil
}

// fixFile applies fixes to a file and returns the fixed content.
func (f *Fixer) fixFile(fset *token.FileSet, file *ast.File, src []byte, report *detector.Report) ([]byte, error) {
	// Collect structs that need reordering
	det := detector.NewDetector(fset, f.config)
	structs := det.CollectStructMethods(file)

	// Filter to only structs that need reordering
	needsReorder := make(map[string]*detector.StructMethods)
	for name, sm := range structs {
		if sm.NeedsReordering() {
			needsReorder[name] = sm
		}
	}

	if len(needsReorder) == 0 {
		return src, nil
	}

	// Reorder the methods
	reorderer := NewReorderer(fset)
	return reorderer.ReorderStructMethods(file, src, needsReorder)
}

// FormatDiff generates a unified diff between original and fixed content.
func FormatDiff(filePath string, original, fixed []byte) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	buf.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))

	// Simple line-by-line diff
	origLines := bytes.Split(original, []byte("\n"))
	fixedLines := bytes.Split(fixed, []byte("\n"))

	_ = origLines  // Used in future diff implementation
	_ = fixedLines // Used in future diff implementation

	buf.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(origLines), len(fixedLines)))
	// Note: This is a simplified diff - for production use, consider
	// using a proper diff library

	return buf.String()
}
