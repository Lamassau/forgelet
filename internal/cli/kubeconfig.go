package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Extract kubeconfig",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if err := os.MkdirAll(cfg.KubeConfigDir, 0755); err != nil {
			return err
		}

		hostIP, err := k0sIP(cfg)
		if err != nil {
			return err
		}

		raw, err := runK0SExecOutput(cfg, "sudo", "k0s", "kubeconfig", "admin")
		if err != nil {
			return err
		}

		content := raw
		if cfg.K0SMode == "vm" {
			content = strings.ReplaceAll(content, "https://localhost:6443", fmt.Sprintf("https://%s:6443", hostIP))
			content = strings.ReplaceAll(content, "https://127.0.0.1:6443", fmt.Sprintf("https://%s:6443", hostIP))
		}

		if err := os.WriteFile(cfg.KubeConfigPath, []byte(content), 0600); err != nil {
			return err
		}

		if err := runCommand("", "kubectl", "--kubeconfig", cfg.KubeConfigPath, "cluster-info"); err != nil {
			return fmt.Errorf("cannot reach API server at https://%s:6443", hostIP)
		}

		abs, _ := filepath.Abs(cfg.KubeConfigPath)
		fmt.Printf("Kubeconfig saved: %s\n", abs)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(kubeconfigCmd)
}
