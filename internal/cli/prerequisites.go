package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var prerequisitesCmd = &cobra.Command{
	Use:   "prerequisites",
	Short: "Install required tools (auto-detects OS)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Printf("Checking prerequisites for %s\n", cfg.Platform)
		switch cfg.Platform {
		case "darwin":
			if !commandExists("brew") {
				return fmt.Errorf("homebrew not found; install from https://brew.sh")
			}
			for _, pkg := range []string{"podman", "kubectl", "k0sctl", "node", "pnpm", "skaffold", "yq"} {
				if err := runCommand("", "brew", "list", pkg); err != nil {
					if err := runCommand("", "brew", "install", pkg); err != nil {
						return err
					}
				}
			}
		case "wsl":
			if commandExists("apt-get") {
				if err := runCommand("", "sudo", "apt-get", "update", "-qq"); err != nil {
					return err
				}
				if err := runCommand("", "sudo", "apt-get", "install", "-y", "curl", "tar", "podman", "kubectl", "nodejs", "pnpm", "yq"); err != nil {
					return err
				}
			}
			if !commandExists("skaffold") {
				if err := runCommand("", "bash", "-lc", "curl -Lo /tmp/skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 && sudo install /tmp/skaffold /usr/local/bin/skaffold && rm -f /tmp/skaffold"); err != nil {
					return err
				}
			}
		case "linux":
			if commandExists("dnf") {
				if err := runCommand("", "sudo", "dnf", "install", "-y", "podman", "kubectl", "curl", "tar", "nodejs", "pnpm", "yq"); err != nil {
					return err
				}
			} else if commandExists("apt-get") {
				if err := runCommand("", "sudo", "apt-get", "update", "-qq"); err != nil {
					return err
				}
				if err := runCommand("", "sudo", "apt-get", "install", "-y", "podman", "kubectl", "curl", "tar", "nodejs", "pnpm", "yq"); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("no supported package manager found (dnf or apt-get)")
			}
			if !commandExists("skaffold") {
				if err := runCommand("", "bash", "-lc", "curl -Lo /tmp/skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 && sudo install /tmp/skaffold /usr/local/bin/skaffold && rm -f /tmp/skaffold"); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported platform: %s", cfg.Platform)
		}

		if !commandExists("cdk8s") {
			if err := runCommand("", "pnpm", "install", "-g", "cdk8s-cli"); err != nil {
				return err
			}
		}

		fmt.Println("All prerequisites installed")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(prerequisitesCmd)
}
