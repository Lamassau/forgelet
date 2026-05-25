package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Full bootstrap (new developer? start here)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("CODESPACES") == "true" {
			fmt.Println("GitHub Codespaces detected. Running in native Linux mode.")
		}

		type namedStep struct {
			name  string
			retry string
			fn    func() error
		}

		steps := []namedStep{
			{"prerequisites", "forgelet prerequisites", func() error { return prerequisitesCmd.RunE(prerequisitesCmd, nil) }},
			{"machine-up", "forgelet machine-up", func() error { return machineUpCmd.RunE(machineUpCmd, nil) }},
			{"k0s-install", "forgelet k0s-install", func() error { return k0sInstallCmd.RunE(k0sInstallCmd, nil) }},
			{"kubeconfig", "forgelet kubeconfig", func() error { return kubeconfigCmd.RunE(kubeconfigCmd, nil) }},
			{"build", "forgelet build", func() error { return buildCmd.RunE(buildCmd, nil) }},
			{"deploy", "forgelet deploy", func() error { return deployCmd.RunE(deployCmd, nil) }},
			{"dns", "forgelet dns", func() error { return dnsCmd.RunE(dnsCmd, nil) }},
		}

		total := time.Now()
		for _, s := range steps {
			step("Running: %s", s.name)
			started := time.Now()
			if err := s.fn(); err != nil {
				printError("Step '%s' failed after %s: %v", s.name, time.Since(started).Round(time.Millisecond), err)
				fmt.Fprintf(os.Stderr, "\nRetry with: %s\n", s.retry)
				return err
			}
			success("%s completed in %s", s.name, time.Since(started).Round(time.Millisecond))
		}

		success("Environment ready in %s!", time.Since(total).Round(time.Second))
		fmt.Println("Run 'forgelet dev' to start the Skaffold inner dev loop.")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(upCmd)
}
