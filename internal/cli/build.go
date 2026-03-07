package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build images",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		environment := cfg.BuildEnv
		if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
			environment = strings.TrimSpace(args[0])
		}

		fmt.Printf("Building images for environment: %s (tag: %s)\n", environment, cfg.Version)
		for _, service := range cfg.Services {
			resolvedImage := resolveVarRef(service.Image, cfg.AppName, service.Name)
			tag := firstNonEmpty(service.Tags, service.Tag, cfg.Version)
			if tag == "" {
				tag = "local"
			}

			target := service.DevTarget
			if environment == "prod" {
				target = service.ProdTarget
			}
			if strings.TrimSpace(target) == "" {
				if environment == "prod" {
					target = "prod"
				} else {
					target = "dev"
				}
			}

			contextPath := filepath.Clean(filepath.Join(cfg.ProjectDir, service.Context))
			dockerfilePath := filepath.Join(contextPath, service.Dockerfile)
			desc := firstNonEmpty(service.Description, service.Name)

			fmt.Printf("Building %s (%s:%s)\n", desc, resolvedImage, tag)
			buildArgs := []string{
				"build",
				"-t", fmt.Sprintf("%s:%s", resolvedImage, tag),
				"-t", fmt.Sprintf("%s/%s:%s", cfg.DockerRegistry, resolvedImage, tag),
				"--format=docker",
				"-f", dockerfilePath,
			}
			if strings.TrimSpace(target) != "" {
				buildArgs = append(buildArgs, "--target", target)
			}
			buildArgs = append(buildArgs, contextPath)
			if err := runCommand("", "podman", buildArgs...); err != nil {
				return err
			}

			fullImage := fmt.Sprintf("%s/%s:%s", cfg.DockerRegistry, resolvedImage, tag)
			if err := runCommand("", "podman", "push", "--tls-verify=false", fullImage); err != nil {
				return err
			}

			if err := importImage(cfg, fullImage); err != nil {
				return err
			}
		}

		namespace := cfg.BuildEnv
		for _, service := range cfg.Services {
			deploy := fmt.Sprintf("%s-deployment", service.Name)
			if err := runKctl(cfg, "get", "deployment", deploy, "-n", namespace); err == nil {
				_ = runKctl(cfg, "rollout", "restart", fmt.Sprintf("deployment/%s", deploy), "-n", namespace)
			}
		}

		fmt.Println("Build complete")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(buildCmd)
}
