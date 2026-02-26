// Package config provides configuration types for the funcorder-fix tool.
package config

// Config holds the configuration for the funcorder-fix tool.
type Config struct {
	// Fix enables automatic fixing of violations.
	Fix bool

	// Write writes the fixed content back to the file.
	Write bool

	// Diff displays a diff of the changes.
	Diff bool

	// List lists files with violations without fixing.
	List bool

	// Verbose enables verbose output.
	Verbose bool

	// CheckConstructor enables checking that constructors (New*, Must*, Or*)
	// appear after struct definition.
	CheckConstructor bool

	// CheckExported enables checking that exported methods appear before
	// unexported methods.
	CheckExported bool
}

// DefaultConfig returns a Config with default settings.
func DefaultConfig() *Config {
	return &Config{
		Fix:              false,
		Write:            false,
		Diff:             false,
		List:             false,
		Verbose:          false,
		CheckConstructor: true,
		CheckExported:    true,
	}
}

// ViolationType represents the type of funcorder violation.
type ViolationType int

const (
	// ViolationConstructor indicates a constructor is not placed after struct.
	ViolationConstructor ViolationType = iota

	// ViolationExported indicates unexported method appears before exported.
	ViolationExported
)

// String returns a human-readable description of the violation type.
func (v ViolationType) String() string {
	switch v {
	case ViolationConstructor:
		return "constructor ordering"
	case ViolationExported:
		return "exported before unexported"
	default:
		return "unknown violation"
	}
}
