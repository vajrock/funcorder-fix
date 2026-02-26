package fixer

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"

	"github.com/vajrock/funcorder-fix/internal/detector"
)

// Reorderer handles reordering of methods in a file according to funcorder rules.
type Reorderer struct {
	fset *token.FileSet
}

// NewReorderer creates a new Reorderer.
func NewReorderer(fset *token.FileSet) *Reorderer {
	return &Reorderer{fset: fset}
}

// structRegion describes the method blocks of a single struct that needs reordering.
type structRegion struct {
	name   string
	sm     *detector.StructMethods
	blocks []MethodBlock // in original source order
}

// slotReplacement is a single byte-range swap: replace src[start:end] with text.
type slotReplacement struct {
	start int
	end   int
	text  string
}

// ReorderStructMethods reorders methods for all structs in the file.
// It uses per-slot byte splicing so that non-method content (standalone functions,
// blank lines, etc.) interleaved between a struct's methods is preserved unchanged.
func (r *Reorderer) ReorderStructMethods(file *ast.File, src []byte, structs map[string]*detector.StructMethods) ([]byte, error) {
	// Quick check
	needsReordering := false
	for _, sm := range structs {
		if sm.NeedsReordering() {
			needsReordering = true
			break
		}
	}
	if !needsReordering {
		return src, nil
	}

	cp := NewCommentPreserver(r.fset, file)

	// Collect per-slot replacements for every struct that needs reordering.
	var replacements []slotReplacement
	for _, sm := range structs {
		if !sm.NeedsReordering() {
			continue
		}
		region, err := r.buildStructRegion(cp, sm, src)
		if err != nil {
			return nil, fmt.Errorf("build region for %s: %w", sm.StructName, err)
		}
		reps, err := r.buildSlotReplacements(region)
		if err != nil {
			return nil, fmt.Errorf("slot replacements for %s: %w", sm.StructName, err)
		}
		replacements = append(replacements, reps...)
	}

	// Process in descending start-offset order so earlier offsets stay valid.
	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start > replacements[j].start
	})

	result := append([]byte(nil), src...)
	for _, rep := range replacements {
		result = spliceBytes(result, rep.start, rep.end, []byte(rep.text))
	}
	return result, nil
}

// buildStructRegion builds MethodBlocks for all methods of sm (in source order).
func (r *Reorderer) buildStructRegion(cp *CommentPreserver, sm *detector.StructMethods, src []byte) (structRegion, error) {
	if len(sm.Methods) == 0 {
		return structRegion{}, fmt.Errorf("struct %s has no methods", sm.StructName)
	}

	blocks := make([]MethodBlock, len(sm.Methods))
	for i, mi := range sm.Methods {
		blocks[i] = cp.GetMethodBlock(mi.FuncDecl, src)
	}

	return structRegion{
		name:   sm.StructName,
		sm:     sm,
		blocks: blocks,
	}, nil
}

// buildSlotReplacements returns one slotReplacement per method.
// Slot i (the byte range of the i-th method in source order) receives the raw text
// of the method that belongs at position i in the expected order.
func (r *Reorderer) buildSlotReplacements(region structRegion) ([]slotReplacement, error) {
	// Build name → rawText lookup from blocks (original source order).
	byName := make(map[string]string, len(region.blocks))
	for _, b := range region.blocks {
		byName[b.Name] = b.RawText
	}

	expectedOrder := region.sm.GetExpectedOrder()
	if len(expectedOrder) != len(region.blocks) {
		return nil, fmt.Errorf("method count mismatch: %d expected vs %d blocks", len(expectedOrder), len(region.blocks))
	}

	reps := make([]slotReplacement, len(region.blocks))
	for i, block := range region.blocks {
		newText, ok := byName[expectedOrder[i].Name]
		if !ok {
			return nil, fmt.Errorf("method %s not found in source map", expectedOrder[i].Name)
		}
		reps[i] = slotReplacement{
			start: r.fset.Position(block.StartPos).Offset,
			end:   r.fset.Position(block.EndPos).Offset,
			text:  newText,
		}
	}
	return reps, nil
}

// spliceBytes replaces src[start:end] with replacement.
func spliceBytes(src []byte, start, end int, replacement []byte) []byte {
	result := make([]byte, 0, len(src)-(end-start)+len(replacement))
	result = append(result, src[:start]...)
	result = append(result, replacement...)
	result = append(result, src[end:]...)
	return result
}

// GetReceiverTypeName is a helper that calls the detector package function.
// This is re-exported for convenience.
var GetReceiverTypeName = detector.GetReceiverTypeName
