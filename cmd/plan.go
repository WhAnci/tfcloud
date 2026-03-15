package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WhAnci/tfcloud/pkg/runner"
)

var (
	planFile      string
	planOutputDir string
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate Terraform files and run terraform init/plan",
	Long:  `Generate Terraform HCL from a YAML manifest, then run terraform init and plan.`,
	RunE:  runPlan,
}

func init() {
	planCmd.Flags().StringVarP(&planFile, "file", "f", "", "Path to the YAML manifest file (required)")
	planCmd.Flags().StringVarP(&planOutputDir, "output", "o", "./output", "Output directory for generated .tf files")
	planCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	// Step 1: Generate
	if err := Generate(planFile, planOutputDir); err != nil {
		return err
	}

	// Step 2: Run terraform
	r := runner.NewRunner(planOutputDir)
	if err := r.CheckTerraform(); err != nil {
		return err
	}

	fmt.Println("\nRunning terraform init...")
	if err := r.Init(); err != nil {
		return err
	}

	fmt.Println("\nRunning terraform plan...")
	return r.Plan()
}
