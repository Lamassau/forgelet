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
		namespace := namespaceForEnv(cfg, cfg.BuildEnv)

		if cfg.DockerRegistry != "" {
			if err := runCommand("", "podman", "start", "registry"); err != nil {
				_ = runCommand("", "podman", "run", "-d", "-p", "5000:5000", "--restart", "always", "--name", "registry", "registry:3")
			}
		}

		if cfg.Platform == "linux" && cfg.K0SMode == "native" && cfg.BuildEnv == "local" {
			coreDNSUpstreams := firstNonEmpty(os.Getenv("CORE_DNS_UPSTREAMS"), "1.1.1.1 8.8.8.8")
			corefile, _ := runKctlOutput(cfg, "get", "configmap", "coredns", "-n", "kube-system", "-o", "jsonpath={.data.Corefile}")
			if corefile != "" && strings.Contains(corefile, "forward . /etc/resolv.conf") {
				patched := strings.ReplaceAll(corefile, "forward . /etc/resolv.conf", fmt.Sprintf("forward . %s", coreDNSUpstreams))
				indented := "    " + strings.ReplaceAll(patched, "\n", "\n    ")
				manifest := fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: coredns\n  namespace: kube-system\ndata:\n  Corefile: |\n%s\n", indented)
				if err := applyManifest(cfg, manifest); err == nil {
					_ = runKctl(cfg, "-n", "kube-system", "rollout", "restart", "deployment/coredns")
					_ = runKctl(cfg, "-n", "kube-system", "rollout", "status", "deployment/coredns", "--timeout=120s")
				}
			}
		}

		if err := runKctl(cfg, "get", "sc", "local-path"); err != nil {
			if err := runKctl(cfg, "apply", "-f", "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.30/deploy/local-path-storage.yaml"); err != nil {
				return err
			}
			_ = runKctl(cfg, "-n", "local-path-storage", "wait", "deploy", "local-path-provisioner", "--for=condition=available", "--timeout=120s")
		}

		if err := runKctl(cfg, "apply", "--validate=false", "-f", fmt.Sprintf("https://raw.githubusercontent.com/metallb/metallb/%s/config/manifests/metallb-native.yaml", cfg.MetallbVersion)); err != nil {
			return err
		}
		_ = runKctl(cfg, "-n", "metallb-system", "wait", "deploy", "controller", "--for=condition=available", "--timeout=120s")
		time.Sleep(5 * time.Second)

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

		_ = applyEnvSecrets(cfg)

		if err := runCommandWithEnv(cfg.InfraDir, envWithBuildEnv(cfg.BuildEnv), "pnpm", "install", "--silent"); err != nil {
			return err
		}
		synthEnv := append(envWithBuildEnv(cfg.BuildEnv), fmt.Sprintf("METALLB_POOL_RANGE=%s", cfg.MetallbPool))
		if err := runCommandWithEnv(cfg.InfraDir, synthEnv, "npx", "cdk8s", "synth"); err != nil {
			return err
		}

		if err := runKctl(cfg, "apply", "-f", fmt.Sprintf("%s/dist/metallb-config.k8s.yaml", cfg.InfraDir)); err != nil {
			return err
		}
		if err := runKctl(cfg, "apply", "-f", fmt.Sprintf("%s/dist/traefik.k8s.yaml", cfg.InfraDir)); err != nil {
			return err
		}

		pendingPVCs, _ := runKctlOutput(cfg, "get", "pvc", "-n", namespace, "--field-selector=status.phase=Pending", "-o", "jsonpath={.items[*].metadata.name}")
		if strings.TrimSpace(pendingPVCs) != "" {
			for _, pvc := range strings.Fields(pendingPVCs) {
				_ = runKctl(cfg, "delete", "pvc", pvc, "-n", namespace)
			}
			time.Sleep(5 * time.Second)
		}

		_ = runKctl(cfg, "apply", "-f", fmt.Sprintf("%s/dist/app.k8s.yaml", cfg.InfraDir))

		for _, deploy := range []string{"api-deployment", "web-deployment"} {
			if err := runKctl(cfg, "get", "deployment", deploy, "-n", namespace); err == nil {
				_ = runKctl(cfg, "rollout", "restart", "deployment/"+deploy, "-n", namespace)
			}
		}

		for _, image := range []string{"docker.io/library/mariadb:10.11", "docker.io/library/mongo:7", "docker.io/valkey/valkey:7", "docker.io/clidey/whodb:latest"} {
			_ = runCommand("", "podman", "pull", image)
			_ = importImage(cfg, image)
		}

		_ = runKctl(cfg, "-n", namespace, "wait", "deploy/api-deployment", "deploy/web-deployment", "--for=condition=available", "--timeout=300s")

		fmt.Print("Waiting for Traefik LoadBalancer IP")
		for i := 0; i < 30; i++ {
			ip, _ := runKctlOutput(cfg, "get", "svc", "traefik", "-n", "traefik-system", "-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
			if strings.TrimSpace(ip) != "" {
				fmt.Printf("\nTraefik LoadBalancer IP: %s\n", strings.TrimSpace(ip))
				if os.Getenv("CODESPACES") == "true" {
					fmt.Printf("Codespace detected! Ensure ports 80/443 are forwarded in the 'Ports' tab, or view your services through the Codespace URL.\n")
				}
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
