package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initciCmd = &cobra.Command{
	Use:   "init-ci",
	Short: "Generate GitHub Actions CI/CD workflow for Forgelet",
	Long:  `Generate a boilerplate GitHub Actions workflow file that builds and deploys using Forgelet.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		workflowDir := filepath.Join(cfg.ProjectDir, ".github", "workflows")
		if err := os.MkdirAll(workflowDir, 0755); err != nil {
			return fmt.Errorf("failed to create .github/workflows directory: %w", err)
		}

		workflowFile := filepath.Join(workflowDir, "forgelet-build.yml")

		// Check if file already exists
		if _, err := os.Stat(workflowFile); err == nil {
			overwrite, _ := cmd.Flags().GetBool("force")
			if !overwrite {
				return fmt.Errorf("workflow file already exists at %s. Use --force to overwrite", workflowFile)
			}
		}

		workflowContent := `name: Forgelet Build

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

env:
  GO_VERSION: '1.21'
  forgelet_ENV: prod

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}

      - name: Install Forgelet CLI
        run: |
          go install github.com/yourusername/forgelet/cmd/forgelet@latest
          # Or build from source if in the same repo:
          # go build -o forgelet ./cmd/forgelet
          # sudo mv forgelet /usr/local/bin/

      - name: Install prerequisites
        run: |
          # Install required tools (podman, kubectl, etc.)
          # Adjust based on your actual prerequisites
          forgelet prerequisites || true

      - name: Build images
        run: |
          forgelet build prod

      - name: Run tests (optional)
        run: |
          # Add your test commands here
          # go test ./...

      # Uncomment the following steps if you want to deploy from CI
      # - name: Set up Kubernetes cluster
      #   run: |
      #     # Set up your Kubernetes cluster or configure kubectl
      #     # This depends on your deployment target (GKE, EKS, etc.)

      # - name: Deploy with Forgelet
      #   run: |
      #     forgelet deploy
      #   env:
      #     KUBECONFIG: ${{ secrets.KUBECONFIG }}

  # Optional: Add a separate deploy job for production
  # deploy-production:
  #   needs: build
  #   runs-on: ubuntu-latest
  #   if: github.ref == 'refs/heads/main'
  #
  #   steps:
  #     - name: Checkout code
  #       uses: actions/checkout@v4
  #
  #     - name: Deploy to production
  #       run: |
  #         # Your deployment steps
  #         echo "Deploying to production..."
`

		if err := os.WriteFile(workflowFile, []byte(workflowContent), 0644); err != nil {
			return fmt.Errorf("failed to write workflow file: %w", err)
		}

		fmt.Printf("✓ GitHub Actions workflow created at: %s\n", workflowFile)
		fmt.Println("\nNext steps:")
		fmt.Println("1. Review and customize the generated workflow file")
		fmt.Println("2. Update the Go version and Forgelet installation method as needed")
		fmt.Println("3. Configure any secrets in your GitHub repository settings")
		fmt.Println("4. Commit and push the workflow file to enable CI/CD")

		return nil
	},
}

func init() {
	initciCmd.Flags().Bool("force", false, "Overwrite existing workflow file if it exists")
	RootCmd.AddCommand(initciCmd)
}
