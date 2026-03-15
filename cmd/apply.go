package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WhAnci/tfcloud/pkg/runner"
)

var (
	applyFile        string
	applyOutputDir   string
	applyAutoApprove bool
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Generate Terraform files and run terraform init/apply",
	Long:  `Generate Terraform HCL from a YAML manifest, then run terraform init and apply.`,
	RunE:  runApply,
}

func init() {
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "Path to the YAML manifest file (required)")
	applyCmd.Flags().StringVarP(&applyOutputDir, "output", "o", "./output", "Output directory for generated .tf files")
	applyCmd.Flags().BoolVar(&applyAutoApprove, "auto-approve", false, "Skip interactive approval for terraform apply")
	applyCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	// Step 1: Generate
	if err := Generate(applyFile, applyOutputDir); err != nil {
		return err
	}

	// Step 2: Run terraform
	r := runner.NewRunner(applyOutputDir)
	if err := r.CheckTerraform(); err != nil {
		return err
	}

	fmt.Println("\nRunning terraform init...")
	if err := r.Init(); err != nil {
		return err
	}

	fmt.Println("\nRunning terraform apply...")
	return r.Apply(applyAutoApprove)
}
