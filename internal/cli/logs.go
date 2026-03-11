package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [service-name]",
	Short: "Tail logs for a service deployment",
	Long:  `Tail logs for a service deployment without needing to specify kubectl namespace or deployment syntax.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		serviceName := args[0]

		// Map service name to deployment name
		// Common convention: service-name -> service-name-deployment
		deploymentName := serviceName
		if deploymentName != "" && deploymentName[len(deploymentName)-1:] != "-deployment" {
			deploymentName = serviceName + "-deployment"
		}

		fmt.Printf("Tailing logs for deployment/%s in namespace %s...\n", deploymentName, cfg.BuildEnv)
		fmt.Println("(Press Ctrl+C to stop)")
		fmt.Println()

		// Get flags
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt64("tail")
		previous, _ := cmd.Flags().GetBool("previous")

		// Build kubectl logs arguments
		args = []string{"logs"}

		if follow {
			args = append(args, "-f")
		}

		if tail > 0 {
			args = append(args, fmt.Sprintf("--tail=%d", tail))
		}

		if previous {
			args = append(args, "--previous")
		}

		args = append(args, fmt.Sprintf("deployment/%s", deploymentName))
		args = append(args, "-n", cfg.BuildEnv)

		// Run kubectl logs
		return runKctl(cfg, args...)
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", true, "Follow log output (stream logs)")
	logsCmd.Flags().Int64("tail", 100, "Number of recent log lines to show (0 for all)")
	logsCmd.Flags().BoolP("previous", "p", false, "Show logs from previous container instance")

	RootCmd.AddCommand(logsCmd)
}
