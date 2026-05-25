package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set via -ldflags at build time
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("forgelet %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
	RootCmd.Version = Version
}
