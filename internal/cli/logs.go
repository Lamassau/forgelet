package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [service-name...]",
	Short: "Tail logs for one or more service deployments",
	Long:  `Tail logs for service deployments without needing to specify kubectl namespace or deployment syntax.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt64("tail")
		previous, _ := cmd.Flags().GetBool("previous")
		container, _ := cmd.Flags().GetString("container")
		namespace := namespaceForEnv(cfg, cfg.BuildEnv)

		buildLogArgs := func(serviceName string) []string {
			deploymentName := serviceName
			if !strings.HasSuffix(deploymentName, "-deployment") {
				deploymentName = serviceName + "-deployment"
			}
			logArgs := []string{"logs"}
			if follow {
				logArgs = append(logArgs, "-f")
			}
			if tail > 0 {
				logArgs = append(logArgs, fmt.Sprintf("--tail=%d", tail))
			}
			if previous {
				logArgs = append(logArgs, "--previous")
			}
			if container != "" {
				logArgs = append(logArgs, "-c", container)
			}
			return append(logArgs, fmt.Sprintf("deployment/%s", deploymentName), "-n", namespace)
		}

		if len(args) == 1 {
			serviceName := args[0]
			deploymentName := serviceName
			if !strings.HasSuffix(deploymentName, "-deployment") {
				deploymentName = serviceName + "-deployment"
			}
			fmt.Printf("Tailing logs for deployment/%s in namespace %s...\n", deploymentName, namespace)
			fmt.Println("(Press Ctrl+C to stop)")
			fmt.Println()
			return runKctl(cfg, buildLogArgs(serviceName)...)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		var wg sync.WaitGroup
		errCh := make(chan error, len(args))
		for _, serviceName := range args {
			serviceName := serviceName
			wg.Add(1)
			go func() {
				defer wg.Done()
				cmdArgs := buildLogArgs(serviceName)
				var command *exec.Cmd
				if cfg.BuildEnv == "local" {
					command = exec.CommandContext(ctx, "sudo", append([]string{"k0s", "kubectl"}, cmdArgs...)...)
				} else {
					command = exec.CommandContext(ctx, "kubectl", cmdArgs...)
				}
				stdout := &prefixWriter{prefix: serviceName, w: os.Stdout}
				stderr := &prefixWriter{prefix: serviceName, w: os.Stderr}
				command.Stdout = stdout
				command.Stderr = stderr
				err := command.Run()
				_ = stdout.Flush()
				_ = stderr.Flush()
				if err != nil && ctx.Err() == nil {
					errCh <- fmt.Errorf("%s: %w", serviceName, err)
				}
			}()
		}
		wg.Wait()
		close(errCh)

		var errs []error
		for err := range errCh {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return errs[0]
		}
		return nil
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", true, "Follow log output (stream logs)")
	logsCmd.Flags().Int64("tail", 100, "Number of recent log lines to show (0 for all)")
	logsCmd.Flags().BoolP("previous", "p", false, "Show logs from previous container instance")
	logsCmd.Flags().StringP("container", "c", "", "Container name within the pod")
	RootCmd.AddCommand(logsCmd)
}
