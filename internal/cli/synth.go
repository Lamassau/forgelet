package cli

import "github.com/spf13/cobra"

var synthCmd = &cobra.Command{
	Use:   "synth",
	Short: "Synthesize CDK8s manifests",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if err := runCommand(cfg.InfraDir, "pnpm", "install", "--silent"); err != nil {
			return err
		}
		return runCommand(cfg.InfraDir, "npx", "cdk8s", "synth")
	},
}

func init() {
	RootCmd.AddCommand(synthCmd)
}
