package config

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Fix {
		t.Error("expected Fix=false")
	}
	if cfg.Write {
		t.Error("expected Write=false")
	}
	if cfg.Diff {
		t.Error("expected Diff=false")
	}
	if cfg.List {
		t.Error("expected List=false")
	}
	if cfg.Verbose {
		t.Error("expected Verbose=false")
	}
	if !cfg.CheckConstructor {
		t.Error("expected CheckConstructor=true")
	}
	if !cfg.CheckExported {
		t.Error("expected CheckExported=true")
	}
}

func TestViolationType_String(t *testing.T) {
	tests := []struct {
		name string
		v    ViolationType
		want string
	}{
		{"constructor", ViolationConstructor, "constructor ordering"},
		{"exported", ViolationExported, "exported before unexported"},
		{"unknown", ViolationType(99), "unknown violation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.String()
			if got != tt.want {
				t.Errorf("ViolationType(%d).String() = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}
