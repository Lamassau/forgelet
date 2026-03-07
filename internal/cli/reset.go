package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Wipe k0s and redeploy (keeps runtime)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Printf("Resetting k0s (%s mode)\n", cfg.K0SMode)
		_ = runK0SSudo(cfg, "k0s", "stop")
		_ = runK0SSudo(cfg, "k0s", "reset")
		_ = runK0SSudo(cfg, "rm", "-rf", "/var/lib/k0s", "/etc/k0s")
		_ = os.Remove(cfg.KubeConfigPath)

		return runSteps(
			func() error { return k0sInstallCmd.RunE(k0sInstallCmd, nil) },
			func() error { return kubeconfigCmd.RunE(kubeconfigCmd, nil) },
			func() error { return buildCmd.RunE(buildCmd, nil) },
			func() error { return deployCmd.RunE(deployCmd, nil) },
		)
	},
}

func init() {
	RootCmd.AddCommand(resetCmd)
}
