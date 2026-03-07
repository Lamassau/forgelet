package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Full bootstrap (new developer? start here)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Full bootstrap - setting up local k0s environment...")

		err := runSteps(
			func() error { return prerequisitesCmd.RunE(prerequisitesCmd, nil) },
			func() error { return machineUpCmd.RunE(machineUpCmd, nil) },
			func() error { return k0sInstallCmd.RunE(k0sInstallCmd, nil) },
			func() error { return kubeconfigCmd.RunE(kubeconfigCmd, nil) },
			func() error { return buildCmd.RunE(buildCmd, nil) },
			func() error { return deployCmd.RunE(deployCmd, nil) },
			func() error { return dnsCmd.RunE(dnsCmd, nil) },
		)
		if err != nil {
			return err
		}

		fmt.Println("\nEnvironment ready!")
		fmt.Println("Run 'forgelet dev' to start the Skaffold inner dev loop.")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(upCmd)
}
