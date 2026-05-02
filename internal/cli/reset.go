package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Wipe k0s but keep the runtime",
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

		if cfg.K0SMode == "vm" {
			fmt.Println("k0s wiped. The Podman Machine is still running.")
		} else {
			fmt.Println("k0s wiped.")
		}
		fmt.Println("Run 'forgelet k0s-install' then 'forgelet deploy' to rebuild.")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(resetCmd)
}
