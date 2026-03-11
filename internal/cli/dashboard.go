package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open Kubernetes Dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Println("Deploying Kubernetes Dashboard...")
		err = runKctl(cfg, "apply", "-f", "https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml")
		if err != nil {
			return fmt.Errorf("failed to deploy dashboard: %v", err)
		}

		fmt.Println("Setting up admin service account...")
		saManifest := `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: forgelet-admin
  namespace: kubernetes-dashboard
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: forgelet-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: forgelet-admin
  namespace: kubernetes-dashboard
`
		// Apply the SA manifest
		if cfg.BuildEnv == "local" {
			// runKctl wrapper uses sudo k0s kubectl
			applyCmd := exec.Command("sudo", "k0s", "kubectl", "apply", "-f", "-")
			applyCmd.Stdin = strings.NewReader(saManifest)
			if err := applyCmd.Run(); err != nil {
				return fmt.Errorf("failed to apply service account: %v", err)
			}
		} else {
			applyCmd := exec.Command("kubectl", "apply", "-f", "-")
			applyCmd.Stdin = strings.NewReader(saManifest)
			if err := applyCmd.Run(); err != nil {
				return fmt.Errorf("failed to apply service account: %v", err)
			}
		}

		fmt.Println("Waiting for dashboard to be ready...")
		_ = runKctl(cfg, "wait", "deploy/kubernetes-dashboard", "-n", "kubernetes-dashboard", "--for=condition=available", "--timeout=120s")

		fmt.Println("Generating token...")
		tokenCmdArgs := []string{"create", "token", "forgelet-admin", "-n", "kubernetes-dashboard", "--duration=24h"}
		token, err := runKctlOutput(cfg, tokenCmdArgs...)
		if err != nil {
			fmt.Println("Warning: Could not create token. Older k8s version? Try 'kubectl -n kubernetes-dashboard create token forgelet-admin' manually.")
		}

		fmt.Println("\n=======================================================")
		fmt.Println("Kubernetes Dashboard Token (valid for 24h):")
		fmt.Println(token)
		fmt.Println("=======================================================")

		fmt.Println("Starting kubectl proxy on http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/ (Ctrl+C to stop)...")

		url := "http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/"

		// Attempt to open browser
		go func() {
			time.Sleep(2 * time.Second)
			openBrowser(url)
		}()

		// Start proxy in foreground
		return runKctl(cfg, "proxy")
	},
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		// Just ignore
	}
}

func init() {
	RootCmd.AddCommand(dashboardCmd)
}
