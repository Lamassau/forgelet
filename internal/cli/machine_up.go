package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var machineUpCmd = &cobra.Command{
	Use:   "machine-up",
	Short: "Start Podman Machine / prepare runtime",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if cfg.K0SMode == "native" {
			fmt.Println("Native mode - no Podman machine required")
			if os.Getenv("CODESPACES") != "true" {
				_ = runCommand("", "systemctl", "--user", "start", "podman.socket")
			} else {
				fmt.Println("Codespace detected - skipping systemctl for podman.socket")
			}
			return nil
		}

		if err := runCommand("", "podman", "machine", "inspect", cfg.ClusterName); err == nil {
			state, stateErr := runCommandOutput("", "podman", "machine", "inspect", cfg.ClusterName, "--format", "{{.State}}")
			if stateErr != nil {
				return stateErr
			}
			if state != "running" {
				if err := runCommand("", "podman", "machine", "start", cfg.ClusterName); err != nil {
					return err
				}
			}
		} else {
			if err := runCommand("", "podman", "machine", "init", cfg.ClusterName, "--cpus", fmt.Sprint(cfg.MachineCPUs), "--memory", fmt.Sprint(cfg.MachineMemory), "--disk-size", fmt.Sprint(cfg.MachineDisk), "--rootful"); err != nil {
				return err
			}
			if err := runCommand("", "podman", "machine", "start", cfg.ClusterName); err != nil {
				return err
			}
		}

		_ = runCommand("", "podman", "system", "connection", "default", cfg.ClusterName)
		ip, _ := k0sIP(cfg)
		fmt.Printf("k0s host ready (IP: %s)\n", ip)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(machineUpCmd)
}
