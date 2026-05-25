package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update forgelet to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Updating forgelet via go install...")
		if err := runCommand("", "go", "install", "github.com/lnyousif/forgelet/cmd/forgelet@latest"); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		fmt.Println("✓ forgelet updated. Run 'forgelet version' to confirm.")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(updateCmd)
}
