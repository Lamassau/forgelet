package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var importImageCmd = &cobra.Command{
	Use:   "import-image IMAGE",
	Short: "Import a local image into k0s containerd",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if err := importImage(cfg, args[0]); err != nil {
			return err
		}

		fmt.Printf("Image imported: %s\n", args[0])
		return nil
	},
}

func init() {
	RootCmd.AddCommand(importImageCmd)
}
