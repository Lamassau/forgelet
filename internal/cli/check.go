package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run health checks and diagnostics",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Printf("Platform: %s | k0s mode: %s\n\n", cfg.Platform, cfg.K0SMode)

		pass := 0
		warnCount := 0
		fail := 0

		check := func(label string, fn func() (string, bool, bool)) {
			msg, ok, isWarn := fn()
			if ok {
				fmt.Printf("  ✓ %s\n", label)
				pass++
			} else if isWarn {
				fmt.Printf("  ⚠ %s: %s\n", label, msg)
				warnCount++
			} else {
				fmt.Printf("  ✗ %s: %s\n", label, msg)
				fail++
			}
		}

		check("k0s running", func() (string, bool, bool) {
			if err := runK0SSudo(cfg, "k0s", "status"); err != nil {
				return "k0s is not running", false, false
			}
			return "", true, false
		})

		check("cluster reachable via kubectl", func() (string, bool, bool) {
			if err := runKctl(cfg, "cluster-info"); err != nil {
				return "cannot reach API server; check kubeconfig", false, false
			}
			return "", true, false
		})

		check("MetalLB controller available", func() (string, bool, bool) {
			out, err := runKctlOutput(cfg, "get", "deploy", "controller", "-n", "metallb-system", "-o", "jsonpath={.status.availableReplicas}")
			if err != nil || strings.TrimSpace(out) == "0" || strings.TrimSpace(out) == "" {
				return "MetalLB controller not available", false, true
			}
			return "", true, false
		})

		check("Traefik LoadBalancer IP assigned", func() (string, bool, bool) {
			ip, err := runKctlOutput(cfg, "get", "svc", "traefik", "-n", "traefik-system", "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
			if err != nil || strings.TrimSpace(ip) == "" {
				return "Traefik has no external IP yet; run 'forgelet deploy'", false, true
			}
			return "", true, false
		})

		check("no failed pods", func() (string, bool, bool) {
			out, err := runKctlOutput(cfg, "get", "pods", "--all-namespaces", "--field-selector=status.phase=Failed", "-o", "jsonpath={.items[*].metadata.name}")
			if err != nil {
				return "could not list pods", false, true
			}
			if names := strings.Fields(out); len(names) > 0 {
				return fmt.Sprintf("failed pods: %s", strings.Join(names, ", ")), false, true
			}
			return "", true, false
		})

		check("no pending PVCs", func() (string, bool, bool) {
			namespace := namespaceForEnv(cfg, cfg.BuildEnv)
			out, err := runKctlOutput(cfg, "get", "pvc", "-n", namespace, "--field-selector=status.phase=Pending", "-o", "jsonpath={.items[*].metadata.name}")
			if err != nil {
				return "could not list PVCs", false, true
			}
			if names := strings.Fields(out); len(names) > 0 {
				return fmt.Sprintf("pending PVCs: %s", strings.Join(names, ", ")), false, true
			}
			return "", true, false
		})

		check("DNS entries in /etc/hosts", func() (string, bool, bool) {
			if cfg.Platform == "darwin" {
				if _, err := os.Stat("/etc/resolver/" + cfg.Domain); err != nil {
					return "resolver file not found; run 'forgelet dns'", false, true
				}
				return "", true, false
			}
			data, err := os.ReadFile("/etc/hosts")
			if err != nil {
				return "cannot read /etc/hosts", false, true
			}
			marker := fmt.Sprintf("# k0s-%s", cfg.ClusterName)
			if !strings.Contains(string(data), marker) {
				return "DNS entries not in /etc/hosts; run 'forgelet dns'", false, true
			}
			return "", true, false
		})

		fmt.Printf("\nResult: %d passed, %d warnings, %d failed\n", pass, warnCount, fail)
		if fail > 0 {
			return fmt.Errorf("%d health check(s) failed", fail)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(checkCmd)
}
