package generator

import (
	"fmt"

	"github.com/WhAnci/tfcloud/pkg/parser"
)

// Generator is the interface that all resource generators must implement.
type Generator interface {
	// Generate produces Terraform HCL bytes from a parsed manifest.
	Generate(manifest *parser.Manifest) ([]byte, error)
	// Kind returns the resource kind this generator handles.
	Kind() string
	// Filename returns the output filename for the generated terraform.
	Filename() string
}

// registry holds all registered generators by kind.
var registry = map[string]Generator{}

// Register adds a generator to the registry.
func Register(g Generator) {
	registry[g.Kind()] = g
}

// GetGenerator returns the generator for a given kind.
func GetGenerator(kind string) (Generator, error) {
	g, ok := registry[kind]
	if !ok {
		return nil, fmt.Errorf("no generator registered for kind %q", kind)
	}
	return g, nil
}

func init() {
	Register(&VPCGenerator{})
}
