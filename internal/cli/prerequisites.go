package cli

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

func installSkaffold() error {
	return runCommand("", "bash", "-lc", "curl -Lo ./skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 && sudo install ./skaffold /usr/local/bin/skaffold && rm -f ./skaffold")
}

var prerequisitesCmd = &cobra.Command{
	Use:   "prerequisites",
	Short: "Install required tools (auto-detects OS)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		fmt.Printf("Checking prerequisites for %s\n", cfg.Platform)
		switch cfg.Platform {
		case "darwin":
			if !commandExists("brew") {
				return fmt.Errorf("homebrew not found; install from https://brew.sh")
			}
			for _, pkg := range []string{"podman", "kubectl", "k0sctl", "node", "pnpm", "skaffold"} {
				if err := runCommand("", "brew", "list", pkg); err != nil {
					if err := runCommand("", "brew", "install", pkg); err != nil {
						return err
					}
				}
			}
			if !commandExists("pnpm") {
				_ = runCommand("", "bash", "-lc", "corepack enable && corepack prepare pnpm@latest --activate")
			}
		case "wsl":
			if commandExists("apt-get") {
				if err := runCommand("", "sudo", "apt-get", "update", "-qq"); err != nil {
					return err
				}
				if err := runCommand("", "sudo", "apt-get", "install", "-y", "curl", "tar"); err != nil {
					return err
				}
				if !commandExists("podman") {
					if err := runCommand("", "sudo", "apt-get", "install", "-y", "podman"); err != nil {
						return err
					}
				}
				if !commandExists("kubectl") {
					if err := runCommand("", "bash", "-lc", "curl -LO https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl && sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && rm -f kubectl"); err != nil {
						return err
					}
				}
				if !commandExists("node") {
					if err := runCommand("", "bash", "-lc", "curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y nodejs"); err != nil {
						return err
					}
				}
			}
			if !commandExists("skaffold") {
				if err := installSkaffold(); err != nil {
					return err
				}
			}
			if !commandExists("pnpm") {
				_ = runCommand("", "bash", "-lc", "corepack enable && corepack prepare pnpm@latest --activate")
			}
		case "linux":
			if commandExists("dnf") {
				if err := runCommand("", "sudo", "dnf", "install", "-y", "podman", "kubectl", "curl", "tar"); err != nil {
					return err
				}
			} else if commandExists("apt-get") {
				if err := runCommand("", "sudo", "apt-get", "update", "-qq"); err != nil {
					return err
				}
				if err := runCommand("", "sudo", "apt-get", "install", "-y", "podman", "kubectl", "curl", "tar"); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("no supported package manager found (dnf or apt-get)")
			}
			if !commandExists("node") {
				if commandExists("dnf") {
					if err := runCommand("", "sudo", "dnf", "install", "-y", "nodejs", "pnpm"); err != nil {
						return err
					}
				} else if err := runCommand("", "sudo", "apt-get", "install", "-y", "nodejs", "pnpm"); err != nil {
					return err
				}
			}
			if !commandExists("skaffold") {
				if err := installSkaffold(); err != nil {
					return err
				}
			}
			if cfg.K0SMode == "native" && !commandExists("k0s") {
				if err := runCommand("", "bash", "-lc", "curl -sSLf https://get.k0s.sh | sudo sh"); err != nil {
					return err
				}
			}
			if !commandExists("pnpm") {
				_ = runCommand("", "bash", "-lc", "corepack enable && corepack prepare pnpm@latest --activate")
			}
		default:
			return fmt.Errorf("unsupported platform: %s", cfg.Platform)
		}

		if !commandExists("yq") {
			switch cfg.Platform {
			case "darwin":
				if err := runCommand("", "brew", "install", "yq"); err != nil {
					return err
				}
			default:
				arch := "amd64"
				if runtime.GOARCH == "arm64" {
					arch = "arm64"
				}
				if err := runCommand("", "bash", "-lc", fmt.Sprintf("YQ_VERSION=$(curl -fsSL https://api.github.com/repos/mikefarah/yq/releases/latest | grep '\"tag_name\"' | sed -E 's/.*\"([^\"]+)\".*/\\1/') && curl -fsSL https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_%s -o ./yq && chmod +x ./yq && sudo mv ./yq /usr/local/bin/yq", arch)); err != nil {
					return err
				}
			}
		}

		if !commandExists("cdk8s") {
			if err := runCommand("", "pnpm", "install", "-g", "cdk8s-cli"); err != nil {
				return err
			}
		}

		registryUser := firstNonEmpty(os.Getenv("DOCKER_REGISTRY_USERNAME"), os.Getenv("PODMAN_REGISTRY_USERNAME"))
		registryPass := firstNonEmpty(os.Getenv("DOCKER_REGISTRY_PASSWORD"), os.Getenv("PODMAN_REGISTRY_PASSWORD"))
		if cfg.DockerRegistry != "" && registryUser != "" && registryPass != "" {
			if err := runCommand("", "bash", "-lc", fmt.Sprintf("printf '%%s' %q | podman login %s -u %s --password-stdin", registryPass, cfg.DockerRegistry, registryUser)); err != nil {
				return err
			}
		}

		fmt.Println("All prerequisites installed")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(prerequisitesCmd)
}
