package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/WhAnci/tfcloud/pkg/generator"
	"github.com/WhAnci/tfcloud/pkg/parser"
)

var (
	generateFile      string
	generateOutputDir string
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Terraform HCL files from a YAML manifest",
	Long:  `Parse a Kubernetes-style YAML manifest and generate corresponding Terraform .tf files.`,
	RunE:  runGenerate,
}

func init() {
	generateCmd.Flags().StringVarP(&generateFile, "file", "f", "", "Path to the YAML manifest file (required)")
	generateCmd.Flags().StringVarP(&generateOutputDir, "output", "o", "./output", "Output directory for generated .tf files")
	generateCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	return Generate(generateFile, generateOutputDir)
}

// Generate parses a YAML file and generates Terraform HCL in the output directory.
func Generate(yamlFile, outputDir string) error {
	manifest, err := parser.ParseFile(yamlFile)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	gen, err := generator.GetGenerator(manifest.Kind)
	if err != nil {
		return err
	}

	hclBytes, err := gen.Generate(manifest)
	if err != nil {
		return fmt.Errorf("failed to generate terraform: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outPath := filepath.Join(outputDir, gen.Filename())
	if err := os.WriteFile(outPath, hclBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Generated %s (%d bytes)\n", outPath, len(hclBytes))
	fmt.Printf("  Kind:     %s\n", manifest.Kind)
	fmt.Printf("  Name:     %s\n", manifest.Metadata.Name)
	fmt.Printf("  Output:   %s\n", outputDir)

	return nil
}
