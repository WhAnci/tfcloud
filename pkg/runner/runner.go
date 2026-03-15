package runner

import (
	"fmt"
	"os"
	"os/exec"
)

// Runner wraps Terraform CLI commands.
type Runner struct {
	WorkDir string
}

// NewRunner creates a new Runner for the given working directory.
func NewRunner(workDir string) *Runner {
	return &Runner{WorkDir: workDir}
}

// CheckTerraform verifies that terraform is installed and accessible.
func (r *Runner) CheckTerraform() error {
	_, err := exec.LookPath("terraform")
	if err != nil {
		return fmt.Errorf("terraform not found in PATH: %w", err)
	}
	return nil
}

// Init runs terraform init in the working directory.
func (r *Runner) Init(extraArgs ...string) error {
	args := append([]string{"init"}, extraArgs...)
	return r.run(args...)
}

// Plan runs terraform plan in the working directory.
func (r *Runner) Plan(extraArgs ...string) error {
	args := append([]string{"plan"}, extraArgs...)
	return r.run(args...)
}

// Apply runs terraform apply in the working directory.
func (r *Runner) Apply(autoApprove bool, extraArgs ...string) error {
	args := []string{"apply"}
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	args = append(args, extraArgs...)
	return r.run(args...)
}

// Destroy runs terraform destroy in the working directory.
func (r *Runner) Destroy(autoApprove bool, extraArgs ...string) error {
	args := []string{"destroy"}
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	args = append(args, extraArgs...)
	return r.run(args...)
}

// run executes a terraform command with real-time output streaming.
func (r *Runner) run(args ...string) error {
	cmd := exec.Command("terraform", args...)
	cmd.Dir = r.WorkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Printf("Running: terraform %s\n", formatArgs(args))

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("terraform exited with code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run terraform: %w", err)
	}
	return nil
}

func formatArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}
