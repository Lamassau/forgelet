package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove everything (VM + cluster + DNS)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Printf("This will destroy '%s'. Continue? [y/N] ", cfg.ClusterName)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" {
			fmt.Println("Aborted")
			return nil
		}

		if cfg.K0SMode == "vm" {
			_ = runCommand("", "podman", "machine", "stop", cfg.ClusterName)
			_ = runCommand("", "podman", "machine", "rm", "-f", cfg.ClusterName)
		} else {
			_ = runCommand("", "sudo", "k0s", "stop")
			_ = runCommand("", "sudo", "k0s", "reset")
			_ = runCommand("", "sudo", "rm", "-rf", "/var/lib/k0s", "/etc/k0s")
		}

		_ = os.Remove(cfg.KubeConfigPath)

		if cfg.Platform == "darwin" {
			_ = runCommand("", "sudo", "rm", "-f", "/etc/resolver/"+cfg.Domain)
			tld := cfg.Domain[strings.LastIndex(cfg.Domain, ".")+1:]
			if tld != cfg.Domain {
				_ = runCommand("", "sudo", "rm", "-f", "/etc/resolver/"+tld)
			}
		} else {
			marker := fmt.Sprintf("# k0s-%s", cfg.ClusterName)
			_ = updateHostsEntries("/etc/hosts", marker, nil, true)
			if cfg.Platform == "wsl" {
				_ = updateHostsEntries("/mnt/c/Windows/System32/drivers/etc/hosts", marker, nil, false)
			}
			if cfg.Platform == "linux" {
				_ = runCommand("", "sudo", "systemctl", "restart", "systemd-resolved")
			}
		}

		fmt.Println("Environment destroyed. Run 'forgelet up' to start fresh.")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(destroyCmd)
}
