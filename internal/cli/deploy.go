package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy platform services",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if cfg.DockerRegistry != "" {
			if err := runCommand("", "podman", "start", "registry"); err != nil {
				_ = runCommand("", "podman", "run", "-d", "-p", "5000:5000", "--restart", "always", "--name", "registry", "registry:3")
			}
		}

		if err := runKctl(cfg, "apply", "--validate=false", "-f", fmt.Sprintf("https://raw.githubusercontent.com/metallb/metallb/%s/config/manifests/metallb-native.yaml", cfg.MetallbVersion)); err != nil {
			return err
		}
		_ = runKctl(cfg, "-n", "metallb-system", "wait", "deploy", "controller", "--for=condition=available", "--timeout=120s")

		_ = runKctl(cfg, "apply", "--validate=false", "-f", cfg.TraefikCRDURL)
		for _, crd := range []string{"ingressroutes.traefik.io", "middlewares.traefik.io", "tlsoptions.traefik.io"} {
			_ = runKctl(cfg, "wait", "--for=condition=established", "crd/"+crd, "--timeout=60s")
		}

		vmIP, err := k0sIP(cfg)
		if err != nil {
			return err
		}
		if strings.TrimSpace(cfg.MetallbPool) == "" {
			parts := strings.Split(vmIP, ".")
			if len(parts) >= 3 {
				cfg.MetallbPool = fmt.Sprintf("%s.%s.%s.200-%s.%s.%s.220", parts[0], parts[1], parts[2], parts[0], parts[1], parts[2])
			}
		}
		if cfg.MetallbPool != "" {
			_ = os.Setenv("METALLB_POOL_RANGE", cfg.MetallbPool)
		}

		if err := runCommand(cfg.InfraDir, "pnpm", "install", "--silent"); err != nil {
			return err
		}
		if err := runCommand(cfg.InfraDir, "npx", "cdk8s", "synth"); err != nil {
			return err
		}

		if err := runKctl(cfg, "apply", "-f", fmt.Sprintf("%s/dist/metallb-config.k8s.yaml", cfg.InfraDir)); err != nil {
			return err
		}
		if err := runKctl(cfg, "apply", "-f", fmt.Sprintf("%s/dist/traefik.k8s.yaml", cfg.InfraDir)); err != nil {
			return err
		}
		_ = runKctl(cfg, "apply", "-f", fmt.Sprintf("%s/dist/app.k8s.yaml", cfg.InfraDir))

		for _, deploy := range []string{"api-deployment", "web-deployment"} {
			if err := runKctl(cfg, "get", "deployment", deploy, "-n", cfg.BuildEnv); err == nil {
				_ = runKctl(cfg, "rollout", "restart", "deployment/"+deploy, "-n", cfg.BuildEnv)
			}
		}

		for _, image := range []string{"docker.io/library/mariadb:10.11", "docker.io/library/mongo:7", "docker.io/valkey/valkey:7", "docker.io/clidey/whodb:latest"} {
			_ = runCommand("", "podman", "pull", image)
			_ = importImage(cfg, image)
		}

		_ = runKctl(cfg, "-n", cfg.BuildEnv, "wait", "deploy/api-deployment", "deploy/web-deployment", "--for=condition=available", "--timeout=300s")

		fmt.Print("Waiting for Traefik LoadBalancer IP")
		for i := 0; i < 30; i++ {
			ip, _ := runKctlOutput(cfg, "get", "svc", "traefik", "-n", "traefik-system", "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
			if strings.TrimSpace(ip) != "" {
				fmt.Printf("\nTraefik LoadBalancer IP: %s\n", strings.TrimSpace(ip))
				return nil
			}
			fmt.Print(".")
			time.Sleep(3 * time.Second)
		}
		fmt.Println("\nTraefik has no external IP yet")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(deployCmd)
}
