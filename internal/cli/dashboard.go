package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
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
		if err := runKctl(cfg, "apply", "-f", "https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml"); err != nil {
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
		if err := applyManifest(cfg, saManifest); err != nil {
			return fmt.Errorf("failed to apply service account: %v", err)
		}

		fmt.Println("Waiting for dashboard to be ready...")
		if err := runKctl(cfg, "wait", "deploy/kubernetes-dashboard", "-n", "kubernetes-dashboard", "--for=condition=available", "--timeout=120s"); err != nil {
			fmt.Printf("Warning: dashboard deployment not ready: %v\n", err)
		}

		fmt.Println("Generating token...")
		token, err := runKctlOutput(cfg, "create", "token", "forgelet-admin", "-n", "kubernetes-dashboard", "--duration=24h")
		if err != nil {
			fmt.Println("Warning: Could not create token. Older k8s version? Try 'kubectl -n kubernetes-dashboard create token forgelet-admin' manually.")
		}

		fmt.Println("\n=======================================================")
		fmt.Println("Kubernetes Dashboard Token (valid for 24h):")
		fmt.Println(token)
		fmt.Println("=======================================================")

		fmt.Println("Starting kubectl proxy on http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/ (Ctrl+C to stop)...")
		url := "http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/"
		go func() {
			time.Sleep(2 * time.Second)
			openBrowser(url)
		}()

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		var proxyCmd *exec.Cmd
		if cfg.BuildEnv == "local" {
			proxyCmd = exec.CommandContext(ctx, "sudo", "k0s", "kubectl", "proxy")
		} else {
			proxyCmd = exec.CommandContext(ctx, "kubectl", "proxy")
		}
		proxyCmd.Stdout = os.Stdout
		proxyCmd.Stderr = os.Stderr
		proxyCmd.Stdin = os.Stdin
		if err := proxyCmd.Run(); err != nil {
			return fmt.Errorf("failed to run kubectl proxy: %w", err)
		}
		return nil
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
