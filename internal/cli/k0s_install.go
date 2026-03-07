package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var k0sInstallCmd = &cobra.Command{
	Use:   "k0s-install",
	Short: "Install k0s",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if err := runK0SSudo(cfg, "k0s", "status"); err == nil {
			fmt.Println("k0s is already running")
			return nil
		}

		if err := runK0SExec(cfg, "bash", "-lc", "command -v k0s"); err != nil {
			if cfg.K0SVersion != "" {
				if err := runK0SExec(cfg, "bash", "-lc", fmt.Sprintf("curl -sSLf https://get.k0s.sh | sudo K0S_VERSION=%s sh", cfg.K0SVersion)); err != nil {
					return err
				}
			} else {
				if err := runK0SExec(cfg, "bash", "-lc", "curl -sSLf https://get.k0s.sh | sudo sh"); err != nil {
					return err
				}
			}
		}

		hostIP, err := k0sIP(cfg)
		if err != nil {
			return err
		}

		k0sConfig := fmt.Sprintf("apiVersion: k0s.k0sproject.io/v1beta1\nkind: ClusterConfig\nmetadata:\n  name: %s\nspec:\n  api:\n    sans:\n      - %s\n      - 127.0.0.1\n      - localhost\n  network:\n    provider: kuberouter\n    podCIDR: 10.244.0.0/16\n    serviceCIDR: 10.96.0.0/12\n", cfg.ClusterName, hostIP)

		tmpFile := filepath.Join(os.TempDir(), "k0s.yaml")
		if err := os.WriteFile(tmpFile, []byte(k0sConfig), 0644); err != nil {
			return err
		}
		defer os.Remove(tmpFile)

		if cfg.K0SMode == "vm" {
			if err := runK0SExec(cfg, "sudo", "mkdir", "-p", "/etc/k0s"); err != nil {
				return err
			}
			copyCmd := fmt.Sprintf("cat %s | podman machine ssh %s -- sudo tee /etc/k0s/k0s.yaml >/dev/null", tmpFile, cfg.ClusterName)
			if err := runCommand("", "bash", "-lc", copyCmd); err != nil {
				return err
			}
		} else {
			if err := runCommand("", "sudo", "mkdir", "-p", "/etc/k0s"); err != nil {
				return err
			}
			if err := runCommand("", "sudo", "cp", tmpFile, "/etc/k0s/k0s.yaml"); err != nil {
				return err
			}
		}

		if cfg.Platform == "linux" && cfg.K0SMode == "native" {
			if err := runCommand("", "bash", "-lc", "systemctl is-active firewalld >/dev/null"); err == nil {
				_ = runCommand("", "sudo", "firewall-cmd", "--zone=trusted", "--add-source=10.244.0.0/16", "--permanent")
				_ = runCommand("", "sudo", "firewall-cmd", "--zone=trusted", "--add-source=10.96.0.0/12", "--permanent")
				_ = runCommand("", "sudo", "firewall-cmd", "--zone=trusted", "--add-interface=kube-bridge", "--permanent")
				_ = runCommand("", "sudo", "firewall-cmd", "--reload")
			}
		}

		if err := runK0SSudo(cfg, "k0s", "install", "controller", "--single", "--config", "/etc/k0s/k0s.yaml"); err != nil {
			return err
		}
		if err := runK0SSudo(cfg, "k0s", "start"); err != nil {
			return err
		}

		for i := 0; i < 60; i++ {
			nodes, _ := runK0SExecOutput(cfg, "sudo", "k0s", "kubectl", "get", "nodes")
			if strings.Contains(nodes, " Ready") {
				break
			}
			time.Sleep(3 * time.Second)
		}

		return runK0SSudo(cfg, "k0s", "kubectl", "get", "nodes", "-o", "wide")
	},
}

func init() {
	RootCmd.AddCommand(k0sInstallCmd)
}
