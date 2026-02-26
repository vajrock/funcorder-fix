package detector

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"

	"github.com/vajrock/funcorder-fix/internal/config"
)

// Detector analyzes Go source files for funcorder violations.
type Detector struct {
	fset   *token.FileSet
	config *config.Config
}

// NewDetector creates a new Detector with the given file set and configuration.
func NewDetector(fset *token.FileSet, cfg *config.Config) *Detector {
	return &Detector{
		fset:   fset,
		config: cfg,
	}
}

// Detect analyzes a file and returns a report of all violations.
func (d *Detector) Detect(file *ast.File, filePath string) *Report {
	report := &Report{
		FilePath:   filePath,
		Violations: []*Violation{},
	}

	// Collect all struct types and their methods
	structs := d.CollectStructMethods(file)

	// Check each struct for violations
	for _, sm := range structs {
		d.checkStructMethods(sm, report)
	}

	// Sort violations by position
	sort.Slice(report.Violations, func(i, j int) bool {
		return report.Violations[i].MethodPos < report.Violations[j].MethodPos
	})

	return report
}

// CollectStructMethods collects all methods grouped by their receiver type.
// This is a public method that can be used by the fixer.
func (d *Detector) CollectStructMethods(file *ast.File) map[string]*StructMethods {
	return d.collectStructMethods(file)
}

// GetMethodsToReorder returns the methods that need reordering for a struct.
func (d *Detector) GetMethodsToReorder(sm *StructMethods) []*MethodInfo {
	if !sm.NeedsReordering() {
		return sm.Methods
	}
	return sm.GetExpectedOrder()
}

// collectStructMethods collects all methods grouped by their receiver type.
func (d *Detector) collectStructMethods(file *ast.File) map[string]*StructMethods {
	structs := make(map[string]*StructMethods)

	// First, collect all struct type declarations
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
						structs[typeSpec.Name.Name] = &StructMethods{
							StructName: typeSpec.Name.Name,
							StructPos:  typeSpec.Pos(),
							StructEnd:  typeSpec.End(),
							Methods:    []*MethodInfo{},
						}
					}
				}
			}
		}
	}

	// Then, collect all method declarations and group them by receiver
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				receiverType := GetReceiverTypeName(fn.Recv.List[0].Type)
				if sm, exists := structs[receiverType]; exists {
					methodInfo := newMethodInfo(fn)
					sm.Methods = append(sm.Methods, methodInfo)
				}
			}
		}
	}

	// Sort methods by position for each struct and categorize them
	for _, sm := range structs {
		sort.Slice(sm.Methods, func(i, j int) bool {
			return sm.Methods[i].Pos < sm.Methods[j].Pos
		})
		sm.CategorizeMethods()
	}

	return structs
}

// checkStructMethods checks a struct's methods for ordering violations.
func (d *Detector) checkStructMethods(sm *StructMethods, report *Report) {
	if len(sm.Methods) <= 1 {
		return
	}

	// Check constructor ordering (constructors should come after struct)
	if d.config.CheckConstructor {
		d.checkConstructorOrdering(sm, report)
	}

	// Check exported before unexported ordering
	if d.config.CheckExported {
		d.checkExportedOrdering(sm, report)
	}
}

// checkConstructorOrdering checks that constructors appear after struct definition
// and before other methods.
func (d *Detector) checkConstructorOrdering(sm *StructMethods, report *Report) {
	if len(sm.Constructors) == 0 {
		return
	}

	// Constructors should come before non-constructor methods
	for _, constructor := range sm.Constructors {
		// Check if any non-constructor exported method comes before this constructor
		for _, exported := range sm.ExportedMethods {
			if exported.Pos < constructor.Pos {
				// Constructor should come before this exported method
				report.AddViolation(newViolation(
					config.ViolationConstructor,
					d.fset,
					constructor.FuncDecl,
					sm.StructName,
					fmt.Sprintf("constructor %s should appear before exported method %s",
						constructor.Name, exported.Name),
					SuggestedFix{
						TargetPos:   exported.Pos,
						TargetName:  exported.Name,
					},
				))
				break
			}
		}
	}
}

// checkExportedOrdering checks that exported methods appear before unexported methods.
func (d *Detector) checkExportedOrdering(sm *StructMethods, report *Report) {
	// Find all unexported methods that appear before exported methods
	for _, unexported := range sm.UnexportedMethods {
		for _, exported := range sm.ExportedMethods {
			if exported.Pos > unexported.Pos {
				// This unexported method should be moved after the exported method
				report.AddViolation(newViolation(
					config.ViolationExported,
					d.fset,
					unexported.FuncDecl,
					sm.StructName,
					fmt.Sprintf("unexported method %s should appear after exported method %s",
						unexported.Name, exported.Name),
					SuggestedFix{
						TargetPos:   exported.End,
						TargetName:  exported.Name,
					},
				))
				break // Only report once per unexported method
			}
		}
	}
}
