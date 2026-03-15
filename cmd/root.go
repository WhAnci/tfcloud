package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tfcloud",
	Short: "Convert Kubernetes-style YAML manifests into Terraform HCL",
	Long: `tfcloud converts Kubernetes-style YAML manifests into Terraform HCL files
and optionally runs terraform commands (init, plan, apply, destroy).

Define your AWS infrastructure using familiar Kubernetes manifest syntax,
and let tfcloud generate clean, well-structured Terraform code.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
