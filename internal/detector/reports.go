// Package detector provides violation detection for funcorder rules.
package detector

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/vajrock/funcorder-fix/internal/config"
)

// Violation represents a single funcorder rule violation.
type Violation struct {
	// Type of violation (constructor or exported ordering).
	Type config.ViolationType

	// Position of the violating method/function in source.
	Position token.Position

	// MethodPos is the position of the method in the file.
	MethodPos token.Pos

	// StructName is the name of the struct this method belongs to.
	StructName string

	// MethodName is the name of the method that violates the rule.
	MethodName string

	// Message describes the violation.
	Message string

	// SuggestedFix contains the suggested position for the method.
	SuggestedFix SuggestedFix
}

// SuggestedFix contains information about how to fix a violation.
type SuggestedFix struct {
	// TargetPos is the position where the method should be moved.
	TargetPos token.Pos

	// TargetName is the name of the method that should be after/before this one.
	TargetName string
}

// String returns a human-readable representation of the violation.
func (v *Violation) String() string {
	return fmt.Sprintf("%s: %s", v.Position, v.Message)
}

// Report contains all violations found in a file.
type Report struct {
	// FilePath is the path to the file being analyzed.
	FilePath string

	// Violations is a list of all violations found.
	Violations []*Violation
}

// HasViolations returns true if there are any violations.
func (r *Report) HasViolations() bool {
	return len(r.Violations) > 0
}

// AddViolation adds a new violation to the report.
func (r *Report) AddViolation(v *Violation) {
	r.Violations = append(r.Violations, v)
}

// newViolation creates a new Violation with the given parameters.
func newViolation(
	vtype config.ViolationType,
	fset *token.FileSet,
	method *ast.FuncDecl,
	structName, message string,
	suggestedFix SuggestedFix,
) *Violation {
	return &Violation{
		Type:       vtype,
		Position:   fset.Position(method.Pos()),
		MethodPos:  method.Pos(),
		StructName: structName,
		MethodName: method.Name.Name,
		Message:    message,
		SuggestedFix: suggestedFix,
	}
}
