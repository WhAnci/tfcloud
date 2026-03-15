package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/WhAnci/tfcloud/pkg/runner"
)

var (
	destroyOutputDir   string
	destroyAutoApprove bool
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Run terraform destroy in the output directory",
	Long:  `Run terraform destroy to tear down infrastructure managed in the output directory.`,
	RunE:  runDestroy,
}

func init() {
	destroyCmd.Flags().StringVarP(&destroyOutputDir, "output", "o", "./output", "Directory containing terraform files")
	destroyCmd.Flags().BoolVar(&destroyAutoApprove, "auto-approve", false, "Skip interactive approval for terraform destroy")
	rootCmd.AddCommand(destroyCmd)
}

func runDestroy(cmd *cobra.Command, args []string) error {
	r := runner.NewRunner(destroyOutputDir)
	if err := r.CheckTerraform(); err != nil {
		return err
	}

	fmt.Println("Running terraform destroy...")
	return r.Destroy(destroyAutoApprove)
}
