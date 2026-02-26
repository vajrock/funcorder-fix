package fixer

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

// CommentPreserver handles preservation of comments during AST manipulation.
type CommentPreserver struct {
	fset  *token.FileSet
	cmap  ast.CommentMap
	file  *ast.File
}

// NewCommentPreserver creates a new CommentPreserver for the given file.
func NewCommentPreserver(fset *token.FileSet, file *ast.File) *CommentPreserver {
	return &CommentPreserver{
		fset: fset,
		cmap: ast.NewCommentMap(fset, file, file.Comments),
		file: file,
	}
}

// GetCommentsFor returns all comments associated with a node.
func (cp *CommentPreserver) GetCommentsFor(node ast.Node) []*ast.CommentGroup {
	return cp.cmap[node]
}

// GetMethodWithComments extracts a method declaration along with its associated comments.
// Returns the full text including comments.
func (cp *CommentPreserver) GetMethodWithComments(fn *ast.FuncDecl, src []byte) string {
	// Find the start position (including any leading comments)
	start := fn.Pos()

	// Check for associated comments
	if comments := cp.cmap[fn]; len(comments) > 0 {
		// Find the earliest comment position
		for _, cg := range comments {
			if cg.Pos() < start {
				start = cg.Pos()
			}
		}
	}

	// Get the text from start to end of function
	startOffset := cp.fset.Position(start).Offset
	endOffset := cp.fset.Position(fn.End()).Offset

	if startOffset < 0 || endOffset > len(src) || startOffset > endOffset {
		// Fallback: just format the function without comments
		var buf bytes.Buffer
		format.Node(&buf, cp.fset, fn)
		return buf.String()
	}

	return string(src[startOffset:endOffset])
}

// PreserveLeadingComments extracts leading comments from a node.
func (cp *CommentPreserver) PreserveLeadingComments(fn *ast.FuncDecl) string {
	var comments []string
	if commentGroups := cp.cmap[fn]; len(commentGroups) > 0 {
		for _, cg := range commentGroups {
			for _, c := range cg.List {
				comments = append(comments, c.Text)
			}
		}
	}
	return strings.Join(comments, "\n")
}

// RebuildCommentMap rebuilds the comment map after AST modifications.
func (cp *CommentPreserver) RebuildCommentMap(file *ast.File) {
	cp.cmap = ast.NewCommentMap(cp.fset, file, file.Comments)
}

// ParseFileWithComments parses a Go source file preserving comments.
func ParseFileWithComments(fset *token.FileSet, src []byte) (*ast.File, error) {
	return parser.ParseFile(fset, "", src, parser.ParseComments|parser.AllErrors)
}

// FormatNode formats an AST node to a string.
func FormatNode(fset *token.FileSet, node ast.Node) (string, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MethodBlock represents a method with its associated comments and spacing.
type MethodBlock struct {
	// FuncDecl is the method/function AST node.
	FuncDecl *ast.FuncDecl

	// Name is the method name, for lookup convenience.
	Name string

	// LeadingComments are comments that appear before the method.
	LeadingComments []*ast.CommentGroup

	// PrecedingNewlines is the number of blank lines before this block.
	PrecedingNewlines int

	// StartPos is the start position (including comments).
	StartPos token.Pos

	// EndPos is the end position of the method.
	EndPos token.Pos

	// RawText is the original source text for this block.
	RawText string
}

// GetMethodBlock builds a MethodBlock for fn, extracting raw text including
// the official doc comment (fn.Doc). We intentionally use fn.Doc rather than
// cp.cmap[fn] because ast.CommentMap can incorrectly associate inline body
// comments from a preceding function with the next FuncDecl node.
func (cp *CommentPreserver) GetMethodBlock(fn *ast.FuncDecl, src []byte) MethodBlock {
	start := fn.Pos()
	// Extend start to include the doc comment if present.
	if fn.Doc != nil && fn.Doc.Pos() < start {
		start = fn.Doc.Pos()
	}

	startOffset := cp.fset.Position(start).Offset
	endOffset := cp.fset.Position(fn.End()).Offset

	var rawText string
	if startOffset >= 0 && endOffset <= len(src) && startOffset <= endOffset {
		rawText = string(src[startOffset:endOffset])
	} else {
		// Fallback: format without comments
		var buf bytes.Buffer
		format.Node(&buf, cp.fset, fn)
		rawText = buf.String()
		start = fn.Pos()
	}

	return MethodBlock{
		FuncDecl:        fn,
		Name:            fn.Name.Name,
		LeadingComments: cp.cmap[fn],
		StartPos:        start,
		EndPos:          fn.End(),
		RawText:         strings.TrimRight(rawText, "\n"),
	}
}
