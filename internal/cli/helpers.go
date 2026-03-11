package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

type BuildService struct {
	Name        string `mapstructure:"name"`
	Image       string `mapstructure:"image"`
	Description string `mapstructure:"description"`
	Dockerfile  string `mapstructure:"dockerfile"`
	Context     string `mapstructure:"context"`
	DevTarget   string `mapstructure:"devTarget"`
	ProdTarget  string `mapstructure:"prodTarget"`
	Tags        string `mapstructure:"tags"`
	Tag         string `mapstructure:"tag"`
}

type forgeletConfig struct {
	ClusterName    string
	KubeConfigDir  string
	KubeConfigPath string
	AppName        string
	Domain         string
	MachineCPUs    int
	MachineMemory  int
	MachineDisk    int
	DockerRegistry string
	K0SVersion     string
	MetallbVersion string
	MetallbPool    string
	TraefikImage   string
	TraefikCRDURL  string
	Version        string
	BuildEnv       string
	Services       []BuildService
	Platform       string
	K0SMode        string
	ProjectDir     string
	InfraDir       string
}

func findforgeletDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for dir := cwd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, ".forgelet", "forgelet.yaml")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return filepath.Join(dir, ".forgelet"), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	return "", errors.New("could not find .forgelet directory from current path")
}

func detectPlatform() (string, string) {
	if os.Getenv("CODESPACES") == "true" {
		return "linux", "native"
	}

	if runtime.GOOS == "darwin" {
		return "darwin", "vm"
	}

	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/version"); err == nil {
			lower := strings.ToLower(string(data))
			if strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl") {
				return "wsl", "vm"
			}
		}

		mode := strings.TrimSpace(os.Getenv("K0S_MODE"))
		if mode == "vm" {
			return "linux", "vm"
		}
		return "linux", "native"
	}

	return runtime.GOOS, "native"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resolveVarRef(value string, appName string, serviceName string) string {
	resolved := value
	resolved = strings.ReplaceAll(resolved, "${app.name}", appName)
	resolved = strings.ReplaceAll(resolved, "${service.name}", serviceName)
	return resolved
}

func loadConfig() (*forgeletConfig, error) {
	forgeletDir, err := findforgeletDir()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(forgeletDir, "forgelet.yaml"))
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read forgelet.yaml: %w", err)
	}

	platform, mode := detectPlatform()

	clusterName := firstNonEmpty(os.Getenv("CLUSTER_NAME"), v.GetString("cluster.clusterName"), "k0s-dev")
	appName := firstNonEmpty(os.Getenv("APP_NAME"), v.GetString("app.name"), "app-name")
	domainRaw := firstNonEmpty(os.Getenv("DOMAIN"), v.GetString("app.domain"), "app-name.local")
	domain := resolveVarRef(domainRaw, appName, "")

	kubeDir := firstNonEmpty(os.Getenv("KUBECONFIG_DIR"), v.GetString("cluster.kubeConfigDir"), "~/.kube")
	kubeDir = strings.Replace(kubeDir, "~", os.Getenv("HOME"), 1)

	buildEnv := firstNonEmpty(
		os.Getenv("forgelet_ENV"),
		v.GetString("build.Environment"),
		v.GetString("build.defaultEnvironment"),
		"local",
	)

	services := []BuildService{}
	if err := v.UnmarshalKey("build.services", &services); err != nil {
		return nil, fmt.Errorf("failed to parse build.services: %w", err)
	}

	cfg := &forgeletConfig{
		ClusterName:    clusterName,
		KubeConfigDir:  kubeDir,
		KubeConfigPath: filepath.Join(kubeDir, clusterName),
		AppName:        appName,
		Domain:         domain,
		MachineCPUs:    v.GetInt("podman.machine.cpus"),
		MachineMemory:  v.GetInt("podman.machine.memory"),
		MachineDisk:    v.GetInt("podman.machine.disk"),
		DockerRegistry: firstNonEmpty(os.Getenv("DOCKER_REGISTRY"), v.GetString("podman.registry")),
		K0SVersion:     firstNonEmpty(os.Getenv("K0S_VERSION"), v.GetString("k0s.version")),
		MetallbVersion: firstNonEmpty(v.GetString("metallb.version"), "v0.14.9"),
		MetallbPool:    firstNonEmpty(os.Getenv("METALLB_POOL_RANGE"), v.GetString("metallb.poolRange")),
		TraefikImage:   firstNonEmpty(v.GetString("traefik.image"), "traefik:v3.2"),
		TraefikCRDURL: firstNonEmpty(
			v.GetString("traefik.crdUrl"),
			"https://raw.githubusercontent.com/traefik/traefik/v3.2.0/docs/content/reference/dynamic-configuration/kubernetes-crd-definition-v1.yml",
		),
		Version:    firstNonEmpty(os.Getenv("VERSION"), v.GetString("build.version"), "local"),
		BuildEnv:   buildEnv,
		Services:   services,
		Platform:   platform,
		K0SMode:    mode,
		ProjectDir: forgeletDir,
		InfraDir:   filepath.Join(forgeletDir, ".infra"),
	}

	if cfg.MachineCPUs == 0 {
		cfg.MachineCPUs = 4
	}
	if cfg.MachineMemory == 0 {
		cfg.MachineMemory = 8192
	}
	if cfg.MachineDisk == 0 {
		cfg.MachineDisk = 50
	}

	return cfg, nil
}

func runCommand(dir string, name string, args ...string) error {
	command := exec.Command(name, args...)
	if dir != "" {
		command.Dir = dir
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin

	if err := command.Run(); err != nil {
		return fmt.Errorf("failed running %s %s: %w", name, strings.Join(args, " "), err)
	}

	return nil
}

func runSteps(steps ...func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}

	return nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runCommandOutput(dir string, name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	if dir != "" {
		command.Dir = dir
	}
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("failed running %s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func runK0SExec(cfg *forgeletConfig, args ...string) error {
	if cfg.K0SMode == "vm" {
		all := append([]string{"machine", "ssh", cfg.ClusterName, "--"}, args...)
		return runCommand("", "podman", all...)
	}
	return runCommand("", args[0], args[1:]...)
}

func runK0SExecOutput(cfg *forgeletConfig, args ...string) (string, error) {
	if cfg.K0SMode == "vm" {
		all := append([]string{"machine", "ssh", cfg.ClusterName, "--"}, args...)
		return runCommandOutput("", "podman", all...)
	}
	return runCommandOutput("", args[0], args[1:]...)
}

func runK0SSudo(cfg *forgeletConfig, args ...string) error {
	all := append([]string{"sudo"}, args...)
	return runK0SExec(cfg, all...)
}

func k0sIP(cfg *forgeletConfig) (string, error) {
	if cfg.K0SMode == "vm" {
		ip, err := runCommandOutput("", "podman", "machine", "ssh", cfg.ClusterName, "--", "bash", "-lc", "hostname -I | awk '{print $1}'")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(ip), nil
	}
	return "127.0.0.1", nil
}

func kctlArgs(cfg *forgeletConfig, extra ...string) []string {
	if cfg.BuildEnv == "local" {
		args := []string{"k0s", "kubectl"}
		args = append(args, extra...)
		return args
	}
	return extra
}

func runKctl(cfg *forgeletConfig, args ...string) error {
	if cfg.BuildEnv == "local" {
		all := append([]string{"k0s", "kubectl"}, args...)
		return runCommand("", "sudo", all...)
	}
	return runCommand("", "kubectl", args...)
}

func runKctlOutput(cfg *forgeletConfig, args ...string) (string, error) {
	if cfg.BuildEnv == "local" {
		all := append([]string{"k0s", "kubectl"}, args...)
		return runCommandOutput("", "sudo", all...)
	}
	return runCommandOutput("", "kubectl", args...)
}

func importImage(cfg *forgeletConfig, image string) error {
	if cfg.K0SMode == "vm" {
		cmd := fmt.Sprintf("podman save %s | podman machine ssh %s -- sudo k0s ctr images import -", image, cfg.ClusterName)
		return runCommand("", "bash", "-lc", cmd)
	}

	tmpTar := filepath.Join(os.TempDir(), fmt.Sprintf("k0s-image-%d.tar", os.Getpid()))
	if err := runCommand("", "podman", "save", "-o", tmpTar, image); err != nil {
		return err
	}
	defer os.Remove(tmpTar)

	return runCommand("", "sudo", "k0s", "ctr", "images", "import", tmpTar)
}

