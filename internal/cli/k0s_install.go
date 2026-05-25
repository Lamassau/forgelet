package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var k0sInstallCmd = &cobra.Command{
	Use:   "k0s-install",
	Short: "Install k0s",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if err := runK0SSudo(cfg, "k0s", "status"); err == nil {
			fmt.Println("k0s is already running")
			return nil
		}

		if err := runK0SExec(cfg, "bash", "-lc", "command -v k0s"); err != nil {
			if cfg.K0SVersion != "" {
				if err := runK0SExec(cfg, "bash", "-lc", fmt.Sprintf("curl -sSLf https://get.k0s.sh | sudo K0S_VERSION=%s sh", cfg.K0SVersion)); err != nil {
					return err
				}
			} else {
				if err := runK0SExec(cfg, "bash", "-lc", "curl -sSLf https://get.k0s.sh | sudo sh"); err != nil {
					return err
				}
			}
		}

		hostIP, err := k0sIP(cfg)
		if err != nil {
			return err
		}

		k0sConfig, err := buildK0SConfig(cfg.ClusterName, hostIP)
		if err != nil {
			return fmt.Errorf("failed to build k0s config: %w", err)
		}

		if cfg.K0SMode == "vm" {
			if err := runK0SExec(cfg, "sudo", "mkdir", "-p", "/etc/k0s"); err != nil {
				return err
			}
			if err := runCommandWithInput("", k0sConfig, "podman", "machine", "ssh", cfg.ClusterName, "--", "sudo", "tee", "/etc/k0s/k0s.yaml"); err != nil {
				return err
			}
		} else {
			if err := runCommand("", "sudo", "mkdir", "-p", "/etc/k0s"); err != nil {
				return err
			}
			if err := runCommandWithInput("", k0sConfig, "sudo", "tee", "/etc/k0s/k0s.yaml"); err != nil {
				return err
			}
		}

		if cfg.Platform == "linux" && cfg.K0SMode == "native" {
			if err := runCommand("", "bash", "-lc", "systemctl is-active firewalld >/dev/null"); err == nil {
				_ = runCommand("", "sudo", "firewall-cmd", "--zone=trusted", "--add-source=10.244.0.0/16", "--permanent")
				_ = runCommand("", "sudo", "firewall-cmd", "--zone=trusted", "--add-source=10.96.0.0/12", "--permanent")
				_ = runCommand("", "sudo", "firewall-cmd", "--zone=trusted", "--add-interface=kube-bridge", "--permanent")
				_ = runCommand("", "sudo", "firewall-cmd", "--reload")
			}
		}

		if err := runK0SSudo(cfg, "k0s", "install", "controller", "--single", "--config", "/etc/k0s/k0s.yaml"); err != nil {
			return err
		}
		if err := runK0SSudo(cfg, "k0s", "start"); err != nil {
			return err
		}

		nodeReady := false
		for i := 0; i < 60; i++ {
			nodes, _ := runK0SExecOutput(cfg, "sudo", "k0s", "kubectl", "get", "nodes")
			if strings.Contains(nodes, " Ready") {
				nodeReady = true
				break
			}
			fmt.Printf("Waiting for node to be Ready (%d/60)...\n", i+1)
			time.Sleep(3 * time.Second)
		}
		if !nodeReady {
			return fmt.Errorf("k0s node did not become Ready within 180s")
		}
		if cfg.K0SVersion == "" {
			if v, err := runK0SExecOutput(cfg, "k0s", "version"); err == nil {
				fmt.Printf("Tip: pin the k0s version in forgelet.yaml: k0s.version: %s\n", strings.TrimSpace(v))
			}
		}

		if err := runK0SSudo(cfg, "k0s", "kubectl", "get", "nodes", "-o", "wide"); err != nil {
			return err
		}
		fmt.Println("k0s up and running")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(k0sInstallCmd)
}

type k0sClusterConfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		API struct {
			SANs []string `yaml:"sans"`
		} `yaml:"api"`
		Network struct {
			Provider    string `yaml:"provider"`
			PodCIDR     string `yaml:"podCIDR"`
			ServiceCIDR string `yaml:"serviceCIDR"`
		} `yaml:"network"`
	} `yaml:"spec"`
}

func buildK0SConfig(clusterName, hostIP string) (string, error) {
	var cfg k0sClusterConfig
	cfg.APIVersion = "k0s.k0sproject.io/v1beta1"
	cfg.Kind = "ClusterConfig"
	cfg.Metadata.Name = clusterName
	cfg.Spec.API.SANs = []string{hostIP, "127.0.0.1", "localhost"}
	cfg.Spec.Network.Provider = "kuberouter"
	cfg.Spec.Network.PodCIDR = "10.244.0.0/16"
	cfg.Spec.Network.ServiceCIDR = "10.96.0.0/12"

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
