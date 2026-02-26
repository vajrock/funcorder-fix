// Command funcorder-fix provides auto-fixing for funcorder linter violations.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vajrock/funcorder-fix/internal/config"
	"github.com/vajrock/funcorder-fix/internal/fixer"
)

var (
	flagFix          bool
	flagWrite        bool
	flagDiff         bool
	flagList         bool
	flagVerbose      bool
	flagConstructor  bool
	flagNoConstructor bool
	flagExported     bool
	flagNoExported   bool
)

func init() {
	flag.BoolVar(&flagFix, "fix", false, "apply automatic fixes")
	flag.BoolVar(&flagWrite, "w", false, "write result to (source) file instead of stdout")
	flag.BoolVar(&flagDiff, "d", false, "display diffs instead of rewriting files")
	flag.BoolVar(&flagList, "l", false, "list files with violations")
	flag.BoolVar(&flagVerbose, "v", false, "verbose output")
	flag.BoolVar(&flagConstructor, "constructor", true, "check constructor ordering")
	flag.BoolVar(&flagNoConstructor, "no-constructor", false, "disable constructor ordering check")
	flag.BoolVar(&flagExported, "exported", true, "check exported before unexported ordering")
	flag.BoolVar(&flagNoExported, "no-exported", false, "disable exported ordering check")
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [path ...]\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "\nFuncorder-fix automatically fixes funcorder linter violations.")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  # Check for violations")
		fmt.Fprintln(os.Stderr, "  funcorder-fix ./...")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Auto-fix and write back to files")
		fmt.Fprintln(os.Stderr, "  funcorder-fix --fix -w ./...")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Show diff of changes")
		fmt.Fprintln(os.Stderr, "  funcorder-fix --fix -d ./...")
	}

	flag.Parse()

	// Build configuration
	cfg := config.DefaultConfig()
	cfg.Fix = flagFix
	cfg.Write = flagWrite
	cfg.Diff = flagDiff
	cfg.List = flagList
	cfg.Verbose = flagVerbose
	cfg.CheckConstructor = flagConstructor && !flagNoConstructor
	cfg.CheckExported = flagExported && !flagNoExported

	// Get paths to process
	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Create fixer
	f := fixer.NewFixer(cfg)

	// Process all paths
	totalViolations := 0
	totalFixed := 0
	hasErrors := false

	for _, path := range paths {
		results := processPath(f, path, cfg)

		for _, result := range results {
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", result.FilePath, result.Error)
				hasErrors = true
				continue
			}

			if result.Violations > 0 {
				totalViolations += result.Violations

				if cfg.List {
					fmt.Println(result.FilePath)
				} else if cfg.Verbose || !cfg.Fix {
					fmt.Fprintf(os.Stderr, "%s: %d violations\n", result.FilePath, result.Violations)
				}

				if result.Fixed {
					totalFixed++
					if err := f.WriteResult(result); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", result.FilePath, err)
						hasErrors = true
					} else if cfg.Write {
						if cfg.Verbose {
							fmt.Fprintf(os.Stderr, "Fixed: %s\n", result.FilePath)
						}
					}
				}
			}
		}
	}

	// Print summary
	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "\nTotal: %d violations in %d files\n", totalViolations, totalFixed)
	}

	if hasErrors {
		os.Exit(1)
	}
}

// processPath processes a single path (file or directory).
func processPath(f *fixer.Fixer, path string, cfg *config.Config) []*fixer.Result {
	// Expand ... wildcard
	if strings.HasSuffix(path, "/...") {
		dir := strings.TrimSuffix(path, "/...")
		return f.ProcessDirectory(dir)
	}

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		return []*fixer.Result{{
			FilePath: path,
			Error:    fmt.Errorf("cannot access path: %w", err),
		}}
	}

	if info.IsDir() {
		return f.ProcessDirectory(path)
	}

	// Single file
	if filepath.Ext(path) == ".go" {
		return []*fixer.Result{f.ProcessFile(path)}
	}

	return nil
}
