package cli

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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
	ClusterName       string
	KubeConfigDir     string
	KubeConfigPath    string
	AppName           string
	Domain            string
	MachineCPUs       int
	MachineMemory     int
	MachineDisk       int
	DockerRegistry    string
	K0SVersion        string
	MetallbVersion    string
	MetallbPool       string
	TraefikImage      string
	TraefikCRDURL     string
	Version           string
	BuildEnv          string
	Services          []BuildService
	Platform          string
	K0SMode           string
	ProjectDir        string
	ConfigDir         string
	ConfigFile        string
	ConfigExists      bool
	InfraDir          string
	DockerComposeFile string
	AppDeployments    []string
	PlatformImages    []string
	RegistryTLSVerify bool
	LocalPathVersion  string
}

func findProjectRoot() (string, string, string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", "", false, err
	}

	for dir := cwd; ; dir = filepath.Dir(dir) {
		forgeletDevenvFile := filepath.Join(dir, ".devenv", "forgelet.yaml")
		if info, statErr := os.Stat(forgeletDevenvFile); statErr == nil && !info.IsDir() {
			return dir, filepath.Join(dir, ".devenv"), forgeletDevenvFile, true, nil
		}

		forgeletFile := filepath.Join(dir, ".forgelet", "forgelet.yaml")
		if info, statErr := os.Stat(forgeletFile); statErr == nil && !info.IsDir() {
			return dir, filepath.Join(dir, ".forgelet"), forgeletFile, true, nil
		}

		if info, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil && !info.IsDir() {
			return dir, filepath.Join(dir, ".devenv"), filepath.Join(dir, ".devenv", "forgelet.yaml"), false, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	return "", "", "", false, errors.New("could not find project root from current path")
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
	projectDir, configDir, configFile, configExists, err := findProjectRoot()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	if configExists {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filepath.Base(configFile), err)
		}
	}

	platform, mode := detectPlatform()

	clusterName := firstNonEmpty(os.Getenv("CLUSTER_NAME"), v.GetString("cluster.clusterName"), "k0s-dev")
	appName := firstNonEmpty(os.Getenv("APP_NAME"), v.GetString("app.name"), "app-name")
	domainRaw := firstNonEmpty(os.Getenv("DOMAIN"), v.GetString("app.domain"), "app-name.local")
	domain := resolveVarRef(domainRaw, appName, "")

	kubeDir := firstNonEmpty(os.Getenv("KUBECONFIG_DIR"), v.GetString("cluster.kubeConfigDir"), "~/.kube")
	kubeDir = strings.Replace(kubeDir, "~", os.Getenv("HOME"), 1)

	buildEnv := firstNonEmpty(
		os.Getenv("APP_ENV"),
		os.Getenv("DEVENV_ENV"),
		os.Getenv("FORGELET_ENV"),
		os.Getenv("forgelet_ENV"),
		v.GetString("build.Environment"),
		v.GetString("build.defaultEnvironment"),
		"local",
	)

	services := []BuildService{}
	if configExists && v.IsSet("build.services") {
		if err := v.UnmarshalKey("build.services", &services); err != nil {
			return nil, fmt.Errorf("failed to parse build.services: %w", err)
		}
	}

	dockerComposeFile := firstNonEmpty(os.Getenv("DOCKER_COMPOSE_FILE"), filepath.Join(configDir, "docker-compose.yml"))
	if _, statErr := os.Stat(dockerComposeFile); statErr != nil {
		fallbackCompose := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
		if info, fallbackErr := os.Stat(fallbackCompose); fallbackErr == nil && !info.IsDir() {
			dockerComposeFile = fallbackCompose
		}
	}

	infraDir := firstNonEmpty(os.Getenv("INFRA_DIR"), filepath.Join(configDir, ".infra"))
	if !directoryExists(infraDir) {
		infraDir = filepath.Join(projectDir, ".infra")
	}

	cfg := &forgeletConfig{
		ClusterName:       clusterName,
		KubeConfigDir:     kubeDir,
		KubeConfigPath:    filepath.Join(kubeDir, clusterName),
		AppName:           appName,
		Domain:            domain,
		MachineCPUs:       v.GetInt("podman.machine.cpus"),
		MachineMemory:     v.GetInt("podman.machine.memory"),
		MachineDisk:       v.GetInt("podman.machine.disk"),
		DockerRegistry:    firstNonEmpty(os.Getenv("DOCKER_REGISTRY"), v.GetString("podman.registry"), "localhost:5000"),
		K0SVersion:        firstNonEmpty(os.Getenv("K0S_VERSION"), v.GetString("k0s.version")),
		MetallbVersion:    firstNonEmpty(v.GetString("metallb.version"), "v0.14.9"),
		MetallbPool:       firstNonEmpty(os.Getenv("METALLB_POOL_RANGE"), v.GetString("metallb.poolRange")),
		TraefikImage:      firstNonEmpty(v.GetString("traefik.image"), "traefik:v3.2"),
		TraefikCRDURL:     firstNonEmpty(v.GetString("traefik.crdUrl"), "https://raw.githubusercontent.com/traefik/traefik/v3.2.0/docs/content/reference/dynamic-configuration/kubernetes-crd-definition-v1.yml"),
		Version:           firstNonEmpty(os.Getenv("VERSION"), v.GetString("build.version"), buildEnv),
		BuildEnv:          buildEnv,
		Services:          services,
		Platform:          platform,
		K0SMode:           mode,
		ProjectDir:        projectDir,
		ConfigDir:         configDir,
		ConfigFile:        configFile,
		ConfigExists:      configExists,
		InfraDir:          infraDir,
		DockerComposeFile: dockerComposeFile,
		AppDeployments:    v.GetStringSlice("deploy.deployments"),
		PlatformImages:    v.GetStringSlice("deploy.platformImages"),
		RegistryTLSVerify: v.GetBool("podman.registryTLSVerify"),
		LocalPathVersion:  firstNonEmpty(v.GetString("localPath.version"), "v0.0.31"),
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

func namespaceForEnv(cfg *forgeletConfig, environment string) string {
	if strings.TrimSpace(environment) == "" || environment == "local" {
		return "local"
	}
	return fmt.Sprintf("%s-%s", cfg.AppName, environment)
}

func envWithBuildEnv(environment string) []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("APP_ENV=%s", environment))
	env = append(env, fmt.Sprintf("DEVENV_ENV=%s", environment))
	return env
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func discoverBuildServices(cfg *forgeletConfig) ([]BuildService, error) {
	if fileExists(cfg.DockerComposeFile) {
		services, err := parseComposeBuildServices(cfg.DockerComposeFile, cfg.AppName)
		if err != nil {
			return nil, err
		}
		if len(services) > 0 {
			return services, nil
		}
	}

	if len(cfg.Services) > 0 {
		return cfg.Services, nil
	}

	return nil, fmt.Errorf("no build services found in %s or config", cfg.DockerComposeFile)
}

func parseComposeBuildServices(composePath string, appName string) ([]BuildService, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read docker compose file: %w", err)
	}

	var compose struct {
		Services map[string]struct {
			Build any `yaml:"build"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("failed to parse docker compose file: %w", err)
	}

	names := make([]string, 0, len(compose.Services))
	for name := range compose.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	services := make([]BuildService, 0, len(names))
	for _, name := range names {
		service := compose.Services[name]
		if service.Build == nil {
			continue
		}

		buildService := BuildService{
			Name:        name,
			Image:       fmt.Sprintf("%s-%s", appName, name),
			Description: name,
			Dockerfile:  "Dockerfile",
			Context:     ".",
			DevTarget:   "dev",
			ProdTarget:  "prod",
		}

		switch build := service.Build.(type) {
		case string:
			buildService.Context = build
		case map[string]any:
			if context, ok := build["context"].(string); ok && strings.TrimSpace(context) != "" {
				buildService.Context = context
			}
			if dockerfile, ok := build["dockerfile"].(string); ok && strings.TrimSpace(dockerfile) != "" {
				buildService.Dockerfile = dockerfile
			}
		}

		services = append(services, buildService)
	}

	return services, nil
}

type cmdOpts struct {
	Dir   string
	Env   []string
	Input io.Reader
}

func runCmd(opts cmdOpts, name string, args ...string) error {
	command := exec.Command(name, args...)
	if opts.Dir != "" {
		command.Dir = opts.Dir
	}
	if opts.Env != nil {
		command.Env = opts.Env
	}
	if opts.Input != nil {
		command.Stdin = opts.Input
	} else {
		command.Stdin = os.Stdin
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("failed running %s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func runCommand(dir string, name string, args ...string) error {
	return runCmd(cmdOpts{Dir: dir}, name, args...)
}

func runCommandWithEnv(dir string, env []string, name string, args ...string) error {
	return runCmd(cmdOpts{Dir: dir, Env: env}, name, args...)
}

func runCommandWithInput(dir string, input string, name string, args ...string) error {
	return runCmd(cmdOpts{Dir: dir, Input: strings.NewReader(input)}, name, args...)
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

func runPipeline(left *exec.Cmd, right *exec.Cmd) error {
	reader, writer := io.Pipe()
	left.Stdout = writer
	left.Stderr = os.Stderr
	left.Stdin = os.Stdin

	right.Stdin = reader
	right.Stdout = os.Stdout
	right.Stderr = os.Stderr

	if err := right.Start(); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return err
	}

	leftErr := left.Run()
	_ = writer.Close()
	rightErr := right.Wait()
	_ = reader.Close()

	if leftErr != nil {
		return leftErr
	}
	if rightErr != nil {
		return rightErr
	}

	return nil
}

func importImage(cfg *forgeletConfig, image string) error {
	if cfg.K0SMode == "vm" {
		saveCmd := exec.Command("podman", "save", image)
		importCmd := exec.Command("podman", "machine", "ssh", cfg.ClusterName, "--", "sudo", "k0s", "ctr", "images", "import", "-")
		return runPipeline(saveCmd, importCmd)
	}

	if os.Getenv("CODESPACES") == "true" {
		saveCmd := exec.Command("podman", "save", image)
		importCmd := exec.Command("sudo", "k0s", "ctr", "images", "import", "-")
		return runPipeline(saveCmd, importCmd)
	}

	tmpTar := filepath.Join(os.TempDir(), fmt.Sprintf("k0s-image-%d.tar", os.Getpid()))
	if err := runCommand("", "podman", "save", "-o", tmpTar, image); err != nil {
		return err
	}
	defer os.Remove(tmpTar)

	return runCommand("", "sudo", "k0s", "ctr", "images", "import", tmpTar)
}

func updateHostsEntries(targetPath string, marker string, entries []string, useSudo bool) error {
	existing := []byte{}
	if data, err := os.ReadFile(targetPath); err == nil {
		existing = data
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	lines := strings.Split(strings.ReplaceAll(string(existing), "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines)+len(entries))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(line, marker) {
			continue
		}
		filtered = append(filtered, line)
	}
	filtered = append(filtered, entries...)

	content := strings.Join(filtered, "\n") + "\n"
	if !useSudo {
		return os.WriteFile(targetPath, []byte(content), 0644)
	}
	return runCommandWithInput("", content, "sudo", "tee", targetPath)
}

func applyManifest(cfg *forgeletConfig, manifest string) error {
	if cfg.BuildEnv == "local" {
		return runCommandWithInput("", manifest, "sudo", "k0s", "kubectl", "apply", "-f", "-")
	}
	return runCommandWithInput("", manifest, "kubectl", "apply", "-f", "-")
}

func applyEnvSecrets(cfg *forgeletConfig) error {
	envFile := filepath.Join(cfg.ProjectDir, ".env.local")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		envFile = filepath.Join(cfg.ProjectDir, ".env")
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			return nil
		}
	}

	namespace := namespaceForEnv(cfg, cfg.BuildEnv)
	fmt.Printf("Applying secrets from %s to namespace %s\n", filepath.Base(envFile), namespace)

	dryRunArgs := []string{
		"create", "secret", "generic", "forgelet-secrets",
		"--from-env-file=" + envFile,
		"-n", namespace,
		"--dry-run=client", "-o", "yaml",
	}
	var dryRun, apply *exec.Cmd
	if cfg.BuildEnv == "local" {
		dryRun = exec.Command("sudo", append([]string{"k0s", "kubectl"}, dryRunArgs...)...)
		apply = exec.Command("sudo", "k0s", "kubectl", "apply", "-f", "-")
	} else {
		dryRun = exec.Command("kubectl", dryRunArgs...)
		apply = exec.Command("kubectl", "apply", "-f", "-")
	}
	return runPipeline(dryRun, apply)
}

func imageExistsInK0s(cfg *forgeletConfig, image string) bool {
	out, err := runK0SExecOutput(cfg, "sudo", "k0s", "ctr", "images", "ls",
		"--format", "{{.Name}}", "-q")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == image {
			return true
		}
	}
	return false
}

func pnpmInstallIfNeeded(dir string) error {
	pkgInfo, pkgErr := os.Stat(filepath.Join(dir, "package.json"))
	nmInfo, nmErr := os.Stat(filepath.Join(dir, "node_modules"))
	if pkgErr == nil && nmErr == nil && nmInfo.ModTime().After(pkgInfo.ModTime()) {
		return nil
	}
	return runCommand(dir, "pnpm", "install", "--silent")
}

func validateMetalLBPool(pool string) error {
	parts := strings.SplitN(pool, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("metallb pool range %q must be in format START-END", pool)
	}
	if net.ParseIP(parts[0]) == nil {
		return fmt.Errorf("metallb pool start IP %q is not valid", parts[0])
	}
	if net.ParseIP(parts[1]) == nil {
		return fmt.Errorf("metallb pool end IP %q is not valid", parts[1])
	}
	return nil
}
