package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// prefixWriter wraps an io.Writer and prefixes each line with a string.
type prefixWriter struct {
	prefix string
	w      io.Writer
	buf    []byte
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	pw.buf = append(pw.buf, p...)
	for {
		idx := bytes.IndexByte(pw.buf, '\n')
		if idx < 0 {
			break
		}
		line := pw.buf[:idx+1]
		if _, err := fmt.Fprintf(pw.w, "[%s] %s", pw.prefix, line); err != nil {
			return 0, err
		}
		pw.buf = pw.buf[idx+1:]
	}
	return len(p), nil
}

func (pw *prefixWriter) Flush() error {
	if len(pw.buf) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(pw.w, "[%s] %s\n", pw.prefix, pw.buf)
	pw.buf = nil
	return err
}

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
		namespace := namespaceForEnv(cfg, environment)

		services, err := discoverBuildServices(cfg)
		if err != nil {
			return err
		}

		fmt.Printf("Building images for environment: %s (tag: %s)\n", environment, cfg.Version)

		var (
			wg   sync.WaitGroup
			mu   sync.Mutex
			errs []error
		)
		for _, service := range services {
			svc := service
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := buildSingleService(cfg, svc, environment); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if len(errs) > 0 {
			return errors.Join(errs...)
		}

		for _, service := range services {
			deploy := fmt.Sprintf("%s-deployment", service.Name)
			if err := runKctl(cfg, "get", "deployment", deploy, "-n", namespace); err == nil {
				_ = runKctl(cfg, "rollout", "restart", fmt.Sprintf("deployment/%s", deploy), "-n", namespace)
			}
		}

		fmt.Println("Build complete")
		return nil
	},
}

func buildSingleService(cfg *forgeletConfig, service BuildService, environment string) error {
	resolvedImage := resolveVarRef(firstNonEmpty(service.Image, fmt.Sprintf("%s-%s", cfg.AppName, service.Name)), cfg.AppName, service.Name)
	tag := firstNonEmpty(service.Tags, service.Tag, cfg.Version)
	if tag == "" {
		tag = environment
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

	contextPath := service.Context
	if !filepath.IsAbs(contextPath) {
		composeRelative := filepath.Join(filepath.Dir(cfg.DockerComposeFile), contextPath)
		projectRelative := filepath.Join(cfg.ProjectDir, contextPath)
		if directoryExists(composeRelative) {
			contextPath = composeRelative
		} else {
			contextPath = projectRelative
		}
	}
	contextPath = filepath.Clean(contextPath)
	dockerfilePath := filepath.Join(contextPath, service.Dockerfile)
	desc := firstNonEmpty(service.Description, service.Name)
	prefix := service.Name

	fmt.Printf("Building %s (%s:%s)\n", desc, resolvedImage, tag)
	stdout := &prefixWriter{prefix: prefix, w: os.Stdout}
	stderr := &prefixWriter{prefix: prefix, w: os.Stderr}

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
	if err := runCmd(cmdOpts{Stdout: stdout, Stderr: stderr}, "podman", buildArgs...); err != nil {
		_ = stdout.Flush()
		_ = stderr.Flush()
		return err
	}
	_ = stdout.Flush()
	_ = stderr.Flush()

	fullImage := fmt.Sprintf("%s/%s:%s", cfg.DockerRegistry, resolvedImage, tag)
	pushArgs := []string{"push", fullImage}
	if !cfg.RegistryTLSVerify {
		pushArgs = append(pushArgs, "--tls-verify=false")
	}
	stdout = &prefixWriter{prefix: prefix, w: os.Stdout}
	stderr = &prefixWriter{prefix: prefix, w: os.Stderr}
	if err := runCmd(cmdOpts{Stdout: stdout, Stderr: stderr}, "podman", pushArgs...); err != nil {
		_ = stdout.Flush()
		_ = stderr.Flush()
		return err
	}
	_ = stdout.Flush()
	_ = stderr.Flush()

	return importImage(cfg, fullImage)
}

func init() {
	RootCmd.AddCommand(buildCmd)
}
