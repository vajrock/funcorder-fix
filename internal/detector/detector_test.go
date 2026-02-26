package detector_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/vajrock/funcorder-fix/internal/config"
	"github.com/vajrock/funcorder-fix/internal/detector"
)

func parseSource(t *testing.T, src string) (*ast.File, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return file, fset
}

func TestDetect_NoViolations(t *testing.T) {
	const src = `package p
type S struct{}
func (s *S) NewCopy() *S  { return &S{} }
func (s *S) Run()          {}
func (s *S) helper()       {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if report.HasViolations() {
		t.Errorf("expected 0 violations, got %d: %v", len(report.Violations), report.Violations)
	}
}

func TestDetect_ConstructorViolation(t *testing.T) {
	const src = `package p
type S struct{}
func (s *S) Run()          {}
func (s *S) NewCopy() *S   { return &S{} }
func (s *S) helper()       {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if !report.HasViolations() {
		t.Fatal("expected at least 1 violation, got 0")
	}

	foundConstructor := false
	for _, v := range report.Violations {
		if v.Type == config.ViolationConstructor {
			foundConstructor = true
			break
		}
	}
	if !foundConstructor {
		t.Errorf("expected at least one ViolationConstructor, none found in %v", report.Violations)
	}
}

func TestDetect_ExportedViolation(t *testing.T) {
	const src = `package p
type S struct{}
func (s *S) helper()  {}
func (s *S) Run()     {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if !report.HasViolations() {
		t.Fatal("expected at least 1 violation, got 0")
	}

	foundExported := false
	for _, v := range report.Violations {
		if v.Type == config.ViolationExported {
			foundExported = true
			break
		}
	}
	if !foundExported {
		t.Errorf("expected at least one ViolationExported, none found in %v", report.Violations)
	}
}

func TestDetect_MixedViolations(t *testing.T) {
	const src = `package p
type Engine struct{}
func (e *Engine) warmUp()               {}
func (e *Engine) Run()                  {}
func (e *Engine) NewInstance() *Engine  { return &Engine{} }
func (e *Engine) Stop()                 {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if !report.HasViolations() {
		t.Fatal("expected violations, got 0")
	}

	foundConstructor := false
	foundExported := false
	for _, v := range report.Violations {
		if v.Type == config.ViolationConstructor {
			foundConstructor = true
		}
		if v.Type == config.ViolationExported {
			foundExported = true
		}
	}
	if !foundConstructor {
		t.Error("expected at least one ViolationConstructor in mixed violations")
	}
	if !foundExported {
		t.Error("expected at least one ViolationExported in mixed violations")
	}
}

func TestDetect_NoConstructorCheck(t *testing.T) {
	const src = `package p
type S struct{}
func (s *S) Run()          {}
func (s *S) NewCopy() *S   { return &S{} }
func (s *S) helper()       {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	cfg.CheckConstructor = false
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if report.HasViolations() {
		t.Errorf("expected 0 violations with CheckConstructor=false, got %d", len(report.Violations))
	}
}

func TestDetect_NoExportedCheck(t *testing.T) {
	const src = `package p
type S struct{}
func (s *S) helper()  {}
func (s *S) Run()     {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	cfg.CheckExported = false
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if report.HasViolations() {
		t.Errorf("expected 0 violations with CheckExported=false, got %d", len(report.Violations))
	}
}

func TestDetect_SingleMethod(t *testing.T) {
	const src = `package p
type S struct{}
func (s *S) Run() {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if report.HasViolations() {
		t.Errorf("expected 0 violations for single method, got %d", len(report.Violations))
	}
}

func TestDetect_StandaloneFunction(t *testing.T) {
	const src = `package p
func NewFoo() {}
func helper() {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if report.HasViolations() {
		t.Errorf("expected 0 violations for standalone functions, got %d", len(report.Violations))
	}
}

func TestDetect_ViolationPosition(t *testing.T) {
	const src = `package p
type S struct{}
func (s *S) helper()  {}
func (s *S) Run()     {}`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	report := d.Detect(file, "test.go")

	if !report.HasViolations() {
		t.Fatal("expected at least 1 violation, got 0")
	}

	v := report.Violations[0]
	if v.MethodName == "" {
		t.Error("expected non-empty MethodName on violation")
	}
	if v.StructName == "" {
		t.Error("expected non-empty StructName on violation")
	}
	if v.Position.Line <= 0 {
		t.Errorf("expected Position.Line > 0, got %d", v.Position.Line)
	}
}

func TestCollectStructMethods(t *testing.T) {
	const src = `package p
type A struct{}
type B struct{}
func (a *A) First()   {}
func (a *A) Second()  {}
func (b *B) Alpha()   {}
func (b *B) Beta()    {}
`

	file, fset := parseSource(t, src)
	cfg := config.DefaultConfig()
	d := detector.NewDetector(fset, cfg)
	structs := d.CollectStructMethods(file)

	if len(structs) != 2 {
		t.Fatalf("expected 2 structs, got %d", len(structs))
	}

	smA, ok := structs["A"]
	if !ok {
		t.Fatal("expected struct A to be present in map")
	}
	if len(smA.Methods) != 2 {
		t.Errorf("expected 2 methods for struct A, got %d", len(smA.Methods))
	}

	smB, ok := structs["B"]
	if !ok {
		t.Fatal("expected struct B to be present in map")
	}
	if len(smB.Methods) != 2 {
		t.Errorf("expected 2 methods for struct B, got %d", len(smB.Methods))
	}

	// Verify methods are sorted by Pos for struct A
	if len(smA.Methods) == 2 && smA.Methods[0].Pos >= smA.Methods[1].Pos {
		t.Errorf("struct A methods not sorted by Pos: methods[0].Pos=%d >= methods[1].Pos=%d",
			smA.Methods[0].Pos, smA.Methods[1].Pos)
	}

	// Verify methods are sorted by Pos for struct B
	if len(smB.Methods) == 2 && smB.Methods[0].Pos >= smB.Methods[1].Pos {
		t.Errorf("struct B methods not sorted by Pos: methods[0].Pos=%d >= methods[1].Pos=%d",
			smB.Methods[0].Pos, smB.Methods[1].Pos)
	}
}
