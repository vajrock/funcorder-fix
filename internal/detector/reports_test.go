package detector

import (
	"go/token"
	"strings"
	"testing"

	"github.com/vajrock/funcorder-fix/internal/config"
)

func TestReport_HasViolations(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		r := &Report{Violations: []*Violation{}}
		if r.HasViolations() {
			t.Error("expected HasViolations()=false for empty report")
		}
	})

	t.Run("non_empty", func(t *testing.T) {
		r := &Report{
			Violations: []*Violation{
				{MethodName: "foo"},
			},
		}
		if !r.HasViolations() {
			t.Error("expected HasViolations()=true")
		}
	})
}

func TestReport_AddViolation(t *testing.T) {
	r := &Report{Violations: []*Violation{}}

	r.AddViolation(&Violation{MethodName: "a"})
	r.AddViolation(&Violation{MethodName: "b"})
	r.AddViolation(&Violation{MethodName: "c"})

	if len(r.Violations) != 3 {
		t.Errorf("expected 3 violations, got %d", len(r.Violations))
	}
}

func TestViolation_String(t *testing.T) {
	v := &Violation{
		Type: config.ViolationExported,
		Position: token.Position{
			Filename: "foo.go",
			Line:     10,
			Column:   1,
		},
		StructName: "Svc",
		MethodName: "helper",
		Message:    "unexported method helper should appear after exported method Run",
	}

	s := v.String()
	if !strings.Contains(s, "foo.go") {
		t.Errorf("expected String() to contain filename, got %q", s)
	}
	if !strings.Contains(s, "unexported method helper") {
		t.Errorf("expected String() to contain message, got %q", s)
	}
}
