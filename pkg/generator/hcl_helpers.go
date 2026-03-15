package generator

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// parseTraversal converts a dot-separated string like "aws_vpc.main.id"
// into an hcl.Traversal for use with hclwrite.
func parseTraversal(ref string) hcl.Traversal {
	parts := strings.Split(ref, ".")
	if len(parts) == 0 {
		return nil
	}

	traversal := hcl.Traversal{
		hcl.TraverseRoot{Name: parts[0]},
	}
	for _, part := range parts[1:] {
		traversal = append(traversal, hcl.TraverseAttr{Name: part})
	}

	return traversal
}

// tokenForOpenBracket returns the correct token type for `[`.
func tokenForOpenBracket() hclsyntax.TokenType {
	return hclsyntax.TokenOBrack
}

// tokenForCloseBracket returns the correct token type for `]`.
func tokenForCloseBracket() hclsyntax.TokenType {
	return hclsyntax.TokenCBrack
}

// tokenForComma returns the correct token type for `,`.
func tokenForComma() hclsyntax.TokenType {
	return hclsyntax.TokenComma
}

// tokenForNewline returns the correct token type for whitespace/newline.
func tokenForNewline() hclsyntax.TokenType {
	return hclsyntax.TokenNewline
}
