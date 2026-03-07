package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster health summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Printf("Platform: %s | k0s mode: %s\n", cfg.Platform, cfg.K0SMode)
		_ = runK0SSudo(cfg, "k0s", "status")
		_ = runKctl(cfg, "get", "nodes", "-o", "wide")
		_ = runKctl(cfg, "get", "pods", "-n", "metallb-system")
		_ = runKctl(cfg, "get", "pods", "-n", "traefik-system")
		ip, _ := runKctlOutput(cfg, "get", "svc", "-n", "traefik-system", "traefik", "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
		if ip == "" {
			fmt.Println("Traefik LoadBalancer IP: pending")
		} else {
			fmt.Printf("Traefik LoadBalancer IP: %s\n", ip)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
