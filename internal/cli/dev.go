package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Inner dev loop via Skaffold (deploy + watch + cleanup)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if _, err := exec.LookPath("skaffold"); err != nil {
			return fmt.Errorf("skaffold not found. run 'forgelet prerequisites' first")
		}

		if err := runKctl(cfg, "cluster-info"); err != nil {
			return fmt.Errorf("cannot reach cluster. run 'forgelet up' or 'forgelet kubeconfig' first")
		}

		_ = applyEnvSecrets(cfg)

		fmt.Println("Synthesizing CDK8s manifests...")
		if err := pnpmInstallIfNeeded(cfg.InfraDir); err != nil {
			return err
		}
		if err := runCommand(cfg.InfraDir, "npx", "cdk8s", "synth"); err != nil {
			return err
		}

		fmt.Println("Building and pushing app images...")
		if err := buildCmd.RunE(buildCmd, nil); err != nil {
			return err
		}

		if os.Getenv("CODESPACES") == "true" {
			fmt.Println("\nCodespace detected! Skaffold is deploying your inner dev loop.")
			fmt.Println("Any services exposed by Traefik will be port-forwarded by GitHub automatically.")
			fmt.Println("Check the 'Ports' tab in your VSCode / Browser to access your application.")
		}

		fmt.Println("Starting Skaffold dev loop (Ctrl+C to stop and clean up)...")
		return runCommand(cfg.ProjectDir,
			"skaffold",
			"dev",
			"--cleanup=true",
			"--port-forward=false",
			"--status-check=true",
			"--trigger=manual",
			"--auto-build=false",
			"--auto-deploy=true",
		)
	},
}

func init() {
	RootCmd.AddCommand(devCmd)
}
