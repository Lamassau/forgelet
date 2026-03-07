package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "forgelet",
	Short: "Local Kubernetes development environment CLI",
	Long: `forgelet is a CLI for managing a local Kubernetes development environment.
The implementation is fully native Go and does not depend on shell scripts.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do nothing by default
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
