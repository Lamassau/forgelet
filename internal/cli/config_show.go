package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration commands",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show resolved configuration",
	Long:  `Print the effective Forgelet configuration (after env-var overrides and defaults).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "# Note: environment-variable overrides are reflected below.")
		fmt.Print(string(data))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	RootCmd.AddCommand(configCmd)
}
