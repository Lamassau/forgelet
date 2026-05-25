package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new Forgelet project",
	Long:  `Create .devenv/forgelet.yaml and optional skeleton files (skaffold.yaml, .infra CDK8s project).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, _ := cmd.Flags().GetString("app-name")
		domain, _ := cmd.Flags().GetString("domain")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		withSkaffold, _ := cmd.Flags().GetBool("skaffold")
		withInfra, _ := cmd.Flags().GetBool("infra")
		overwrite, _ := cmd.Flags().GetBool("force")

		if appName == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("determining current directory: %w", err)
			}
			appName = filepath.Base(cwd)
		}
		if domain == "" {
			domain = appName + ".local"
		}
		if clusterName == "" {
			clusterName = "k0s-dev"
		}

		devenvDir := filepath.Join(".", ".devenv")
		if err := os.MkdirAll(devenvDir, 0o755); err != nil {
			return fmt.Errorf("creating .devenv: %w", err)
		}

		cfgFile := filepath.Join(devenvDir, "forgelet.yaml")
		if _, err := os.Stat(cfgFile); err == nil && !overwrite {
			return fmt.Errorf("forgelet.yaml already exists at %s; use --force to overwrite", cfgFile)
		}

		content := forgeletYAMLTemplate(appName, domain, clusterName)
		if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing forgelet.yaml: %w", err)
		}
		fmt.Printf("✓ Created %s\n", cfgFile)

		if withSkaffold {
			if err := writeSkaffoldYAML(appName, overwrite); err != nil {
				fmt.Fprintf(os.Stderr, "⚠ Could not write skaffold.yaml: %v\n", err)
			} else {
				fmt.Println("✓ Created skaffold.yaml")
			}
		}

		if withInfra {
			if err := initInfraDir(appName, overwrite); err != nil {
				fmt.Fprintf(os.Stderr, "⚠ Could not scaffold .infra: %v\n", err)
			} else {
				fmt.Println("✓ Created .infra/ CDK8s skeleton")
			}
		}

		fmt.Println("\nNext steps:")
		fmt.Println("  1. Review .devenv/forgelet.yaml")
		fmt.Println("  2. Run: forgelet up")
		return nil
	},
}

func forgeletYAMLTemplate(appName, domain, clusterName string) string {
	return fmt.Sprintf(`# Forgelet configuration
# See: https://github.com/lnyousif/forgelet/docs/config-reference.md

cluster:
  # Name of the k0s cluster / Podman Machine
  clusterName: %s
  # Directory where kubeconfig is stored (~ expands to $HOME)
  kubeConfigDir: ~/.kube

app:
  # Application name — used as namespace prefix and image prefix
  name: %s
  # Local domain for DNS entries (supports ${app.name} variable)
  domain: %s

podman:
  machine:
    # Resources for the Podman VM (macOS / WSL only)
    cpus: 4
    memory: 8192   # MB
    disk: 50       # GB
  # Local container registry
  registry: localhost:5000
  # Set to true to require TLS when pushing to the registry
  registryTLSVerify: false

k0s:
  # Pin a specific k0s version, e.g. v1.30.1+k0s.0 — empty = latest
  version: ""

metallb:
  version: v0.14.9
  # IP pool range for MetalLB LoadBalancer IPs (empty = auto-detect from VM IP)
  poolRange: ""

traefik:
  image: traefik:v3.2.0
  crdUrl: https://raw.githubusercontent.com/traefik/traefik/v3.2.0/docs/content/reference/dynamic-configuration/kubernetes-crd-definition-v1.yml

build:
  # Default build environment: local | dev | prod
  defaultEnvironment: local
  # Image tag to use (default: build environment)
  version: local

# deploy:
#   # Names of Kubernetes deployments to wait for after deploy
#   deployments:
#     - api-deployment
#     - web-deployment
#   # Extra platform images to pull and import into k0s containerd
#   platformImages:
#     - docker.io/library/mariadb:10.11

localPath:
  # Version of rancher/local-path-provisioner
  version: v0.0.31
`, clusterName, appName, domain)
}

func writeSkaffoldYAML(appName string, overwrite bool) error {
	if _, err := os.Stat("skaffold.yaml"); err == nil && !overwrite {
		return fmt.Errorf("skaffold.yaml already exists; use --force to overwrite")
	}
	content := fmt.Sprintf(`apiVersion: skaffold/v4beta11
kind: Config
metadata:
  name: %s
build:
  local:
    push: false
  artifacts: []   # Add your artifacts here

deploy:
  kubectl:
    manifests: []  # Point to your .infra/dist/*.yaml
`, appName)
	return os.WriteFile("skaffold.yaml", []byte(content), 0o644)
}

func initInfraDir(appName string, overwrite bool) error {
	infraDir := ".infra"
	if err := os.MkdirAll(infraDir, 0o755); err != nil {
		return err
	}

	writeFile := func(name, content string) error {
		path := filepath.Join(infraDir, name)
		if _, err := os.Stat(path); err == nil && !overwrite {
			return fmt.Errorf("%s already exists; use --force to overwrite", path)
		}
		return os.WriteFile(path, []byte(content), 0o644)
	}

	packageJSON := fmt.Sprintf(`{
  "name": "%s-infra",
  "version": "1.0.0",
  "description": "CDK8s infrastructure for %s",
  "scripts": {
    "synth": "cdk8s synth"
  },
  "devDependencies": {
    "cdk8s-cli": "^2.0.0",
    "constructs": "^10.0.0",
    "cdk8s": "^2.0.0",
    "cdk8s-plus-27": "^2.0.0",
    "typescript": "^5.0.0",
    "ts-node": "^10.9.2"
  }
}
`, appName, appName)
	if err := writeFile("package.json", packageJSON); err != nil {
		return err
	}
	if err := writeFile("cdk8s.yaml", `language: typescript
app: npx ts-node main.ts
imports:
  - k8s
output: dist
`); err != nil {
		return err
	}
	mainTS := fmt.Sprintf(`import { App } from 'cdk8s';
// Import your chart classes here
// import { AppChart } from './app-chart';

const app = new App();
// new AppChart(app, '%s');
app.synth();
`, appName)
	return writeFile("main.ts", mainTS)
}

func init() {
	initCmd.Flags().String("app-name", "", "Application name (default: current directory name)")
	initCmd.Flags().String("domain", "", "Local domain (default: <app-name>.local)")
	initCmd.Flags().String("cluster-name", "k0s-dev", "Cluster name")
	initCmd.Flags().Bool("skaffold", false, "Generate a skeleton skaffold.yaml")
	initCmd.Flags().Bool("infra", false, "Generate a skeleton .infra/ CDK8s project")
	initCmd.Flags().Bool("force", false, "Overwrite existing files")
	RootCmd.AddCommand(initCmd)
}
