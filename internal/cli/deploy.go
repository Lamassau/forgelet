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
		if err := validateConfig(cfg); err != nil {
			return err
		}
		return runSteps(
			func() error { return deployEnsureRegistry(cfg) },
			func() error { return deployPatchCoreDNS(cfg) },
			func() error { return deployStorageClass(cfg) },
			func() error { return deployMetalLB(cfg) },
			func() error { return deployTraefikCRDs(cfg) },
			func() error { return deployComputeMetalLBPool(cfg) },
			func() error { return deploySynthAndApply(cfg) },
			func() error { return deployPlatformImages(cfg) },
			func() error { return deployWaitForApps(cfg) },
			func() error { return deployWaitForTraefik(cfg) },
		)
	},
}

func deployEnsureRegistry(cfg *forgeletConfig) error {
	if cfg.DockerRegistry == "" {
		return nil
	}
	if err := runCommand("", "podman", "start", "registry"); err != nil {
		_ = runCommand("", "podman", "run", "-d", "-p", "5000:5000", "--restart", "always", "--name", "registry", "registry:3")
	}
	return nil
}

func deployPatchCoreDNS(cfg *forgeletConfig) error {
	if cfg.Platform != "linux" || cfg.K0SMode != "native" || cfg.BuildEnv != "local" {
		return nil
	}
	coreDNSUpstreams := firstNonEmpty(os.Getenv("CORE_DNS_UPSTREAMS"), "1.1.1.1 8.8.8.8")
	corefile, _ := runKctlOutput(cfg, "get", "configmap", "coredns", "-n", "kube-system", "-o", "jsonpath={.data.Corefile}")
	if corefile == "" || !strings.Contains(corefile, "forward . /etc/resolv.conf") {
		return nil
	}
	patched := strings.ReplaceAll(corefile, "forward . /etc/resolv.conf", fmt.Sprintf("forward . %s", coreDNSUpstreams))
	indented := "    " + strings.ReplaceAll(patched, "\n", "\n    ")
	manifest := fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: coredns\n  namespace: kube-system\ndata:\n  Corefile: |\n%s\n", indented)
	if err := applyManifest(cfg, manifest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to patch CoreDNS: %v\n", err)
		return nil
	}
	_ = runKctl(cfg, "-n", "kube-system", "rollout", "restart", "deployment/coredns")
	_ = runKctl(cfg, "-n", "kube-system", "rollout", "status", "deployment/coredns", "--timeout=120s")
	return nil
}

func deployStorageClass(cfg *forgeletConfig) error {
	if err := runKctl(cfg, "get", "sc", "local-path"); err != nil {
		url := fmt.Sprintf("https://raw.githubusercontent.com/rancher/local-path-provisioner/%s/deploy/local-path-storage.yaml", cfg.LocalPathVersion)
		if err := runKctl(cfg, "apply", "-f", url); err != nil {
			return err
		}
		_ = runKctl(cfg, "-n", "local-path-storage", "wait", "deploy", "local-path-provisioner",
			"--for=condition=available", "--timeout=120s")
	}
	if err := runKctl(cfg, "patch", "storageclass", "local-path",
		"-p", `{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not mark local-path as default StorageClass: %v\n", err)
	}
	return nil
}

func deployMetalLB(cfg *forgeletConfig) error {
	url := fmt.Sprintf("https://raw.githubusercontent.com/metallb/metallb/%s/config/manifests/metallb-native.yaml", cfg.MetallbVersion)
	if err := runKctl(cfg, "apply", "--server-side", "--force-conflicts", "-f", url); err != nil {
		return err
	}
	if err := runKctl(cfg, "-n", "metallb-system", "wait", "deploy", "controller",
		"--for=condition=available", "--timeout=120s"); err != nil {
		return fmt.Errorf("metallb controller did not become available: %w", err)
	}
	if err := runKctl(cfg, "-n", "metallb-system", "wait", "pod",
		"-l", "component=webhook-server", "--for=condition=ready", "--timeout=60s"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: metallb webhook pod not ready: %v\n", err)
	}
	return nil
}

func deployTraefikCRDs(cfg *forgeletConfig) error {
	if err := runKctl(cfg, "apply", "--server-side", "--force-conflicts", "-f", cfg.TraefikCRDURL); err != nil {
		return fmt.Errorf("failed to apply Traefik CRDs: %w", err)
	}
	for _, crd := range []string{"ingressroutes.traefik.io", "middlewares.traefik.io", "tlsoptions.traefik.io"} {
		if err := runKctl(cfg, "wait", "--for=condition=established", "crd/"+crd, "--timeout=60s"); err != nil {
			return fmt.Errorf("CRD %s did not become established: %w", crd, err)
		}
	}
	return nil
}

func deployComputeMetalLBPool(cfg *forgeletConfig) error {
	vmIP, err := k0sIP(cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.MetallbPool) == "" {
		parts := strings.Split(vmIP, ".")
		if len(parts) >= 3 {
			cfg.MetallbPool = fmt.Sprintf("%s.%s.%s.200-%s.%s.%s.220",
				parts[0], parts[1], parts[2],
				parts[0], parts[1], parts[2])
		}
	}
	if cfg.MetallbPool != "" {
		if err := validateMetalLBPool(cfg.MetallbPool); err != nil {
			return err
		}
		_ = os.Setenv("METALLB_POOL_RANGE", cfg.MetallbPool)
	}
	return nil
}

func deploySynthAndApply(cfg *forgeletConfig) error {
	namespace := namespaceForEnv(cfg, cfg.BuildEnv)

	_ = applyEnvSecrets(cfg)

	if err := pnpmInstallIfNeeded(cfg.InfraDir); err != nil {
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

	pendingPVCs, _ := runKctlOutput(cfg, "get", "pvc", "-n", namespace,
		"--field-selector=status.phase=Pending", "-o", "jsonpath={.items[*].metadata.name}")
	if strings.TrimSpace(pendingPVCs) != "" {
		for _, pvc := range strings.Fields(pendingPVCs) {
			_ = runKctl(cfg, "delete", "pvc", pvc, "-n", namespace)
		}
		for i := 0; i < 20; i++ {
			remaining, _ := runKctlOutput(cfg, "get", "pvc", "-n", namespace,
				"--field-selector=status.phase=Pending", "-o", "jsonpath={.items[*].metadata.name}")
			if strings.TrimSpace(remaining) == "" {
				break
			}
			fmt.Print(".")
			time.Sleep(time.Second)
		}
		fmt.Println()
	}

	if err := runKctl(cfg, "apply", "-f", fmt.Sprintf("%s/dist/app.k8s.yaml", cfg.InfraDir)); err != nil {
		return fmt.Errorf("failed to apply app manifest: %w", err)
	}

	deployments := cfg.AppDeployments
	if len(deployments) == 0 {
		deployments = []string{"api-deployment", "web-deployment"}
	}
	for _, deploy := range deployments {
		if err := runKctl(cfg, "get", "deployment", deploy, "-n", namespace); err == nil {
			if err := runKctl(cfg, "rollout", "restart", "deployment/"+deploy, "-n", namespace); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not restart %s: %v\n", deploy, err)
			}
		}
	}

	return nil
}

func deployPlatformImages(cfg *forgeletConfig) error {
	images := cfg.PlatformImages
	if len(images) == 0 {
		images = []string{
			"docker.io/library/mariadb:10.11",
			"docker.io/library/mongo:7",
			"docker.io/valkey/valkey:7",
			"docker.io/clidey/whodb:latest",
		}
	}
	for _, image := range images {
		if imageExistsInK0s(cfg, image) {
			fmt.Printf("Image already in containerd, skipping: %s\n", image)
			continue
		}
		if err := runCommand("", "podman", "pull", image); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to pull %s: %v\n", image, err)
			continue
		}
		if err := importImage(cfg, image); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to import %s: %v\n", image, err)
		}
	}
	return nil
}

func deployWaitForApps(cfg *forgeletConfig) error {
	namespace := namespaceForEnv(cfg, cfg.BuildEnv)
	deployments := cfg.AppDeployments
	if len(deployments) == 0 {
		deployments = []string{"api-deployment", "web-deployment"}
	}
	waitArgs := []string{"-n", namespace, "wait"}
	for _, d := range deployments {
		waitArgs = append(waitArgs, "deploy/"+d)
	}
	waitArgs = append(waitArgs, "--for=condition=available", "--timeout=300s")
	_ = runKctl(cfg, waitArgs...)
	return nil
}

func deployWaitForTraefik(cfg *forgeletConfig) error {
	fmt.Print("Waiting for Traefik LoadBalancer IP")
	for i := 0; i < 30; i++ {
		ip, _ := runKctlOutput(cfg, "get", "svc", "traefik", "-n", "traefik-system",
			"-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
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
}

func init() {
	RootCmd.AddCommand(deployCmd)
}
