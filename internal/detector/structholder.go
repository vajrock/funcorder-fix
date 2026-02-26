package detector

import (
	"go/ast"
	"go/token"
	"strings"
)

// MethodInfo holds information about a single method.
type MethodInfo struct {
	// FuncDecl is the AST node for the function declaration.
	FuncDecl *ast.FuncDecl

	// Name is the method/function name.
	Name string

	// IsExported indicates if the method is exported (public).
	IsExported bool

	// IsConstructor indicates if this is a constructor (New*, Must*, Or*).
	IsConstructor bool

	// ReceiverType is the receiver type name for methods, empty for functions.
	ReceiverType string

	// Position in the source file.
	Pos token.Pos

	// End position in the source file.
	End token.Pos

	// DocComment is the documentation comment group (if any).
	DocComment *ast.CommentGroup
}

// StructMethods holds information about all methods of a struct.
type StructMethods struct {
	// StructName is the name of the struct.
	StructName string

	// StructPos is the position of the struct type declaration.
	StructPos token.Pos

	// StructEnd is the end position of the struct type declaration.
	StructEnd token.Pos

	// Methods is a list of all methods belonging to this struct.
	Methods []*MethodInfo

	// Constructors are methods that are constructors (New*, Must*, Or*).
	Constructors []*MethodInfo

	// ExportedMethods are public methods (excluding constructors).
	ExportedMethods []*MethodInfo

	// UnexportedMethods are private methods.
	UnexportedMethods []*MethodInfo
}

// newMethodInfo creates a MethodInfo from an ast.FuncDecl.
func newMethodInfo(fn *ast.FuncDecl) *MethodInfo {
	name := fn.Name.Name
	info := &MethodInfo{
		FuncDecl:    fn,
		Name:        name,
		IsExported:  ast.IsExported(name),
		IsConstructor: isConstructor(name),
		Pos:         fn.Pos(),
		End:         fn.End(),
		DocComment:  fn.Doc,
	}

	// Extract receiver type if this is a method
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		info.ReceiverType = GetReceiverTypeName(fn.Recv.List[0].Type)
	}

	return info
}

// GetReceiverTypeName extracts the type name from a receiver expression.
// Handles both value and pointer receivers.
func GetReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// isConstructor checks if a function/method name matches constructor patterns.
// Constructors are functions that start with New, Must, or Or.
func isConstructor(name string) bool {
	return strings.HasPrefix(name, "New") ||
		strings.HasPrefix(name, "Must") ||
		strings.HasPrefix(name, "Or")
}

// CategorizeMethods separates methods into constructors, exported, and unexported.
func (sm *StructMethods) CategorizeMethods() {
	for _, m := range sm.Methods {
		if m.IsConstructor {
			sm.Constructors = append(sm.Constructors, m)
		} else if m.IsExported {
			sm.ExportedMethods = append(sm.ExportedMethods, m)
		} else {
			sm.UnexportedMethods = append(sm.UnexportedMethods, m)
		}
	}
}

// GetExpectedOrder returns methods in the expected order:
// Constructors → Exported → Unexported
func (sm *StructMethods) GetExpectedOrder() []*MethodInfo {
	result := make([]*MethodInfo, 0, len(sm.Methods))
	result = append(result, sm.Constructors...)
	result = append(result, sm.ExportedMethods...)
	result = append(result, sm.UnexportedMethods...)
	return result
}

// GetCurrentOrder returns methods in their current order (sorted by position).
func (sm *StructMethods) GetCurrentOrder() []*MethodInfo {
	// Methods should already be in position order from parsing
	return sm.Methods
}

// NeedsReordering checks if methods need to be reordered.
func (sm *StructMethods) NeedsReordering() bool {
	if len(sm.Methods) <= 1 {
		return false
	}

	currentOrder := sm.GetCurrentOrder()
	expectedOrder := sm.GetExpectedOrder()

	if len(currentOrder) != len(expectedOrder) {
		return false // Should never happen
	}

	for i := range currentOrder {
		if currentOrder[i].Name != expectedOrder[i].Name {
			return true
		}
	}

	return false
}
