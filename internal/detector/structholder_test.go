package detector

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/vajrock/funcorder-fix/internal/config"
)

func TestGetReceiverTypeName(t *testing.T) {
	tests := []struct {
		name string
		expr ast.Expr
		want string
	}{
		{
			name: "Ident",
			expr: &ast.Ident{Name: "Foo"},
			want: "Foo",
		},
		{
			name: "StarExpr_Ident",
			expr: &ast.StarExpr{X: &ast.Ident{Name: "Foo"}},
			want: "Foo",
		},
		{
			name: "IndexExpr_generic_value",
			expr: &ast.IndexExpr{X: &ast.Ident{Name: "Container"}},
			want: "Container",
		},
		{
			name: "StarExpr_IndexExpr_generic_pointer",
			expr: &ast.StarExpr{X: &ast.IndexExpr{X: &ast.Ident{Name: "Container"}}},
			want: "Container",
		},
		{
			name: "IndexListExpr_multi_type_param",
			expr: &ast.IndexListExpr{X: &ast.Ident{Name: "Map"}},
			want: "Map",
		},
		{
			name: "StarExpr_IndexListExpr",
			expr: &ast.StarExpr{X: &ast.IndexListExpr{X: &ast.Ident{Name: "Map"}}},
			want: "Map",
		},
		{
			name: "nil_fallback",
			expr: &ast.BasicLit{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetReceiverTypeName(tt.expr)
			if got != tt.want {
				t.Errorf("GetReceiverTypeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsConstructor(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"NewSnapshot", true},
		{"MustValidate", true},
		{"OrDefault", true},
		{"Run", false},
		{"helper", false},
		{"new", false},    // lowercase "new" — not a constructor
		{"NEWER", false},  // not HasPrefix("New") — it's "NEW" not "New"
		{"Newsroom", true}, // false positive by design (HasPrefix "New")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConstructor(tt.name)
			if got != tt.want {
				t.Errorf("isConstructor(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetExpectedOrder(t *testing.T) {
	sm := &StructMethods{
		StructName: "Foo",
		Methods: []*MethodInfo{
			{Name: "helper", IsConstructor: false, IsExported: false},
			{Name: "Run", IsConstructor: false, IsExported: true},
			{Name: "NewFoo", IsConstructor: true, IsExported: true},
			{Name: "Stop", IsConstructor: false, IsExported: true},
			{Name: "reset", IsConstructor: false, IsExported: false},
		},
	}
	sm.CategorizeMethods()

	expected := sm.GetExpectedOrder()
	wantNames := []string{"NewFoo", "Run", "Stop", "helper", "reset"}

	if len(expected) != len(wantNames) {
		t.Fatalf("GetExpectedOrder() returned %d methods, want %d", len(expected), len(wantNames))
	}
	for i, m := range expected {
		if m.Name != wantNames[i] {
			t.Errorf("GetExpectedOrder()[%d].Name = %q, want %q", i, m.Name, wantNames[i])
		}
	}
}

func TestNeedsReordering_EdgeCases(t *testing.T) {
	t.Run("zero_methods", func(t *testing.T) {
		sm := &StructMethods{Methods: []*MethodInfo{}}
		sm.CategorizeMethods()
		if sm.NeedsReordering() {
			t.Error("expected NeedsReordering()=false for 0 methods")
		}
	})

	t.Run("one_method", func(t *testing.T) {
		sm := &StructMethods{
			Methods: []*MethodInfo{{Name: "Run", IsExported: true}},
		}
		sm.CategorizeMethods()
		if sm.NeedsReordering() {
			t.Error("expected NeedsReordering()=false for 1 method")
		}
	})

	t.Run("already_ordered", func(t *testing.T) {
		sm := &StructMethods{
			Methods: []*MethodInfo{
				{Name: "Run", IsExported: true},
				{Name: "helper", IsExported: false},
			},
		}
		sm.CategorizeMethods()
		if sm.NeedsReordering() {
			t.Error("expected NeedsReordering()=false for already-ordered methods")
		}
	})

	t.Run("needs_reorder", func(t *testing.T) {
		sm := &StructMethods{
			Methods: []*MethodInfo{
				{Name: "helper", IsExported: false},
				{Name: "Run", IsExported: true},
			},
		}
		sm.CategorizeMethods()
		if !sm.NeedsReordering() {
			t.Error("expected NeedsReordering()=true")
		}
	})
}

func TestCategorizeMethods_ViaDetector(t *testing.T) {
	src := `package foo
type Svc struct{}
func (s *Svc) NewCopy() *Svc { return nil }
func (s *Svc) MustInit() {}
func (s *Svc) Run() {}
func (s *Svc) helper() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := NewDetector(fset, cfg)
	structs := d.CollectStructMethods(file)

	sm, ok := structs["Svc"]
	if !ok {
		t.Fatal("struct Svc not found")
	}

	if len(sm.Constructors) != 2 {
		t.Errorf("expected 2 constructors, got %d", len(sm.Constructors))
	}
	if len(sm.ExportedMethods) != 1 {
		t.Errorf("expected 1 exported method (Run), got %d", len(sm.ExportedMethods))
	}
	if len(sm.UnexportedMethods) != 1 {
		t.Errorf("expected 1 unexported method (helper), got %d", len(sm.UnexportedMethods))
	}
}

func TestCollectStructMethods_Generics(t *testing.T) {
	src := `package foo
type Container[T any] struct{ items []T }
func (c *Container[T]) Add(item T) { c.items = append(c.items, item) }
func (c *Container[T]) reset() { c.items = nil }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := NewDetector(fset, cfg)
	structs := d.CollectStructMethods(file)

	sm, ok := structs["Container"]
	if !ok {
		t.Fatal("struct Container not found — GetReceiverTypeName may not handle generic pointer receivers")
	}
	if len(sm.Methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(sm.Methods))
	}
}
