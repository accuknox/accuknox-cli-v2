package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/push"
	"github.com/spf13/cobra"
)

var pushOptions push.Options

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push policies to knoxctl",
	Long:  "Push policie to knoxctl, only for internal use.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := push.Push(&pushOptions); err != nil {
			return err
		}
		return nil
	},
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringVarP(&pushOptions.GitPATPath, "git-pat", "", "", "Path to git PAT file")
}
