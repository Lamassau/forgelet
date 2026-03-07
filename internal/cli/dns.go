package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Configure local DNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		lbIP, _ := runKctlOutput(cfg, "get", "svc", "traefik", "-n", "traefik-system", "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
		lbIP = strings.TrimSpace(lbIP)
		if lbIP == "" {
			lbIP, err = k0sIP(cfg)
			if err != nil {
				return err
			}
		}

		marker := fmt.Sprintf("# k0s-%s", cfg.ClusterName)
		subdomains := []string{"api", "web", "db", "traefik", "whodb", "dashboard", "grafana", ""}

		switch cfg.Platform {
		case "darwin":
			if err := runCommand("", "sudo", "mkdir", "-p", "/etc/resolver"); err != nil {
				return err
			}
			if err := runCommand("", "bash", "-lc", fmt.Sprintf("echo 'nameserver %s' | sudo tee /etc/resolver/%s >/dev/null", lbIP, cfg.Domain)); err != nil {
				return err
			}
			_ = runCommand("", "sudo", "dscacheutil", "-flushcache")
			_ = runCommand("", "sudo", "killall", "-HUP", "mDNSResponder")
		default:
			_ = runCommand("", "sudo", "sed", "-i", fmt.Sprintf("/%s/d", marker), "/etc/hosts")
			for _, sub := range subdomains {
				host := cfg.Domain
				if sub != "" {
					host = sub + "." + cfg.Domain
				}
				if err := runCommand("", "bash", "-lc", fmt.Sprintf("echo '%s %s %s' | sudo tee -a /etc/hosts >/dev/null", lbIP, host, marker)); err != nil {
					return err
				}
			}
			if cfg.Platform == "linux" {
				_ = runCommand("", "sudo", "systemctl", "restart", "systemd-resolved")
			}
		}

		fmt.Printf("DNS configured. Endpoints: http://web.%s http://api.%s\n", cfg.Domain, cfg.Domain)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(dnsCmd)
}
