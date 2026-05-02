package cli

import (
	"fmt"
	"os"
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
			fmt.Println("Traefik has no LoadBalancer IP yet; falling back to the k0s host IP.")
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
			if err := runCommandWithInput("", fmt.Sprintf("nameserver %s\n", lbIP), "sudo", "tee", "/etc/resolver/"+cfg.Domain); err != nil {
				return err
			}
			tld := cfg.Domain[strings.LastIndex(cfg.Domain, ".")+1:]
			if tld != cfg.Domain {
				_ = runCommandWithInput("", fmt.Sprintf("nameserver %s\n", lbIP), "sudo", "tee", "/etc/resolver/"+tld)
			}
			_ = runCommand("", "sudo", "dscacheutil", "-flushcache")
			_ = runCommand("", "sudo", "killall", "-HUP", "mDNSResponder")
		default:
			entries := make([]string, 0, len(subdomains))
			for _, sub := range subdomains {
				host := cfg.Domain
				if sub != "" {
					host = sub + "." + cfg.Domain
				}
				entries = append(entries, fmt.Sprintf("%s %s %s", lbIP, host, marker))
			}
			if err := updateHostsEntries("/etc/hosts", marker, entries, true); err != nil {
				return err
			}
			if cfg.Platform == "wsl" {
				winHosts := "/mnt/c/Windows/System32/drivers/etc/hosts"
				if _, statErr := os.Stat(winHosts); statErr == nil {
					if err := updateHostsEntries(winHosts, marker, entries, false); err != nil {
						fmt.Printf("Warning: could not update Windows hosts file: %v\n", err)
					}
				}
			}
			if cfg.Platform == "linux" {
				_ = runCommand("", "sudo", "systemctl", "restart", "systemd-resolved")
			}
		}

		fmt.Printf("DNS configured. Endpoints: http://web.%s http://api.%s http://traefik.%s:8080/dashboard/\n", cfg.Domain, cfg.Domain, cfg.Domain)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(dnsCmd)
}
