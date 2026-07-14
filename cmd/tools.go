package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/tools"
	"github.com/spf13/cobra"
)

func init() {
	cfg, err := tools.Load()
	if err != nil {
		// Non-fatal: tools.yaml parse failure should not prevent other commands.
		fmt.Printf("warning: failed to load tools config: %v\n", err)
		return
	}

	for _, t := range cfg.Tools {
		t := t // capture for closure
		cmd := &cobra.Command{
			Use:                t.Name,
			Short:              t.Description,
			DisableFlagParsing: true, // pass all flags/args through to the tool
			Args:               cobra.ArbitraryArgs,
			// Override parent PersistentPreRunE — tools don't need a k8s client.
			PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
			RunE: func(cmd *cobra.Command, args []string) error {
				binPath, err := t.EnsureInstalled()
				if err != nil {
					return err
				}
				args = enrichPkgscanCycloneDXArgs(t.Name, args)
				return tools.Exec(binPath, args)
			},
		}
		rootCmd.AddCommand(cmd)
	}
}
