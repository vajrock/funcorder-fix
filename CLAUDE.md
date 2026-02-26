# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

An **auto-fixer** for the `funcorder` linter (https://github.com/manuelarte/funcorder). The original linter detects method ordering violations but does not support automatic fixes. This tool provides:

1. Detection of funcorder violations (ported from original linter)
2. Auto-fix capability to reorder methods according to funcorder rules
3. CLI interface compatible with golangci-lint conventions

## funcorder Rules Enforced

- **Constructor ordering**: Constructor *methods* (`New*`, `Must*`, `Or*`) should appear before other methods for each struct
- **Exported before unexported**: Public methods must appear before private methods for each struct
- **Standalone functions are gaps**: `func NewFoo() *Foo` (no receiver) is never reordered — only methods with receivers are touched
- **Generic structs supported**: `Container[T any]`, `Map[K, V]` — receiver type is resolved recursively through `StarExpr`/`IndexExpr`/`IndexListExpr`

## Development Commands

```bash
# Build
make build                    # Creates bin/funcorder-fix

# Run all tests
make test                     # go test -v -race ./...

# Run a single test
go test -v -run TestProcessFile_MixedViolations ./internal/fixer/
go test -v -run TestDetect_ConstructorViolation ./internal/detector/

# Fuzz tests (30s)
go test -fuzz=FuzzProcessFile -fuzztime=30s ./internal/fixer/
go test -fuzz=FuzzSpliceBytes -fuzztime=30s ./internal/fixer/

# Lint (funcorder only, examples/input/ excluded intentionally)
make lint                     # golangci-lint run ./...

# Run fixer on example files
make run-example              # Detection only
make run-example-fix          # Fix, output to stdout (verbose goes to stderr)
make run-example-write        # Fix, write back to files

# Verify golden examples pass funcorder
make verify-fix

# Fix the tool's own source (dogfooding)
go run ./cmd/funcorder-fix --fix -w -v ./internal/...
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--fix` | Apply automatic fixes |
| `-w` | Write result back to file |
| `-d` | Display diff instead of rewriting |
| `-l` | List files with violations only |
| `-v` | Verbose output (goes to **stderr**, not stdout) |
| `--no-constructor` | Disable constructor ordering check |
| `--no-exported` | Disable exported/unexported ordering check |

## Testing Workflow

After any change to `internal/fixer/` or `internal/detector/`:

```bash
# 1. Unit tests
go test -v -race ./...

# 2. Regenerate fixed examples
go run ./cmd/funcorder-fix --fix -v ./examples/input/ > examples/golden/crl_service.go

# 3. Verify zero funcorder violations in golden
golangci-lint run --no-config -E funcorder ./examples/golden/...

# 4. Verify examples compile
go build ./examples/...
```

If you change `testdata/src/` files, regenerate `testdata/golden/` too:
```bash
# Files with violations — regenerate via fixer
for f in constructor_only exported_only mixed_violations with_comments \
         with_spacing multi_struct multiline_funcs generics gap_functions; do
  go run ./cmd/funcorder-fix --fix ./testdata/src/${f}.go > testdata/golden/${f}.go
done
# File without violations — copy as-is
cp testdata/src/no_violations.go testdata/golden/no_violations.go
```

## Architecture

```
funcorder-fix/
├── cmd/funcorder-fix/main.go      # CLI entry point + processPath helper
├── internal/
│   ├── config/config.go           # Config struct + ViolationType enum
│   ├── detector/
│   │   ├── detector.go            # Entry point: CollectStructMethods → check violations
│   │   ├── structholder.go        # MethodInfo, StructMethods, GetReceiverTypeName()
│   │   └── reports.go             # Violation and Report types
│   └── fixer/
│       ├── fixer.go               # Orchestrates: detect → collect → reorder → write
│       ├── reorder.go             # Text-splice reordering via spliceBytes()
│       └── comment_preserve.go    # CommentPreserver, MethodBlock, GetMethodBlock()
├── stubs/                          # Stub packages so examples/ can compile
├── examples/
│   ├── input/                     # Real-world file with violations (crl_service.go)
│   └── golden/                    # Generated output (committed for reference)
└── testdata/
    ├── src/                       # 10 test files covering comments, spacing, generics, etc.
    └── golden/                    # Expected fixer output (generated, committed)
```

### Detection Flow

1. Parse file with `parser.ParseComments`
2. `CollectStructMethods`: walk `file.Decls`, collect struct type declarations, then group methods by receiver type; sort each struct's methods by `token.Pos`
3. `CategorizeMethods`: split into Constructors / ExportedMethods / UnexportedMethods
4. Compare `GetCurrentOrder()` vs `GetExpectedOrder()` (Constructors → Exported → Unexported) to detect violations

### Fixing Flow (text-splice, NOT AST printer)

The fixer uses **raw byte splicing** rather than `format.Node()` to avoid Go's AST printer misplacing comments (comment positions are immutable `token.Pos` values that become contradictory after reordering).

Key concept — **per-slot replacement**: for each method in its original source position (slot _i_), replace its byte range with the raw text of the method that belongs at position _i_ in the expected order. Gaps between method slots (standalone helper functions, blank lines) are never touched.

1. `NewCommentPreserver` builds `ast.CommentMap` (used for `LeadingComments` metadata, but **not** for computing `StartPos`)
2. `GetMethodBlock(fn, src)` extracts raw bytes `src[startOffset:endOffset]` where `startOffset` = `fn.Doc.Pos()` (if doc comment exists) else `fn.Pos()`, and `endOffset` = `fn.End()` (one past closing `}`)
3. `buildStructRegion` builds a `[]MethodBlock` in original source order
4. `buildSlotReplacements` pairs each original slot with the expected method's `RawText`
5. All replacements applied in **descending start-offset order** so earlier offsets stay valid after each splice

### Critical: `ast.CommentMap` gotcha

`ast.NewCommentMap` can associate inline body comments from function A with the following `FuncDecl` B (because it uses "innermost enclosing node" rules and a `BlockStmt` can be ambiguous). **Always use `fn.Doc` directly** (the AST field) rather than `cmap[fn]` when looking for a function's leading doc comment. `fn.Doc` is set by the parser and is always correct. `GetMethodBlock()` already follows this rule.

### Pitfall: `filepath.Walk(".")` and hidden-directory filter

`ProcessDirectory` skips directories whose name starts with `.` (hidden dirs like `.git`). However, `filepath.Walk(".")` calls the callback for the root directory first, and `info.Name()` returns `"."` — which starts with a dot. The filter explicitly excludes `"."` and `".."` to avoid skipping the entire walk:

```go
name := info.Name()
if name == "vendor" || (name != "." && name != ".." && len(name) > 0 && name[0] == '.') {
```

This is tested in `TestProcessDirectory_DotPath`, `TestProcessDirectory_DotDotPath`, and `TestProcessDirectory_DotPathStillSkipsHidden`.

## Key Invariants

- `sm.Methods` is always sorted by `token.Pos` (enforced at the end of `collectStructMethods`)
- `GetExpectedOrder()` preserves relative order within each category (constructors, exported, unexported)
- Standalone functions (`func foo()`, no receiver) are never touched by the fixer; they live in the gaps between method slots
- The `stubs/` package exists only to make `examples/` compilable; it contains stub types/interfaces
- **Constructor detection applies to receiver methods only**: `func (s *S) NewCopy() *S` is detected as a constructor; the common Go idiom `func NewS() *S` (no receiver) is a standalone function and lives in a gap — never reordered
- **`GetReceiverTypeName` is recursive**: handles `*Container[T]` → `StarExpr(IndexExpr(Ident))` and `Map[K,V]` → `IndexListExpr(Ident)` by delegating back to itself
