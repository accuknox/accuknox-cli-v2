package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

// getJoinCmd represents the getJoinCmd command
var getJoinCmd = &cobra.Command{
	Use:   "get-join-cmd",
	Short: "Get join command for joining a worker node with the control plane node at the given adddress",
	Long:  "Get join command for joining a worker node with the control plane node at the given adddress",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Failed to generate join command: %s", err)
		}

		// TODO: update as more platforms added
		dockerComposeFilePath := filepath.Join(homeDir, ".accuknox", "docker-compose.yaml")

		// check if the docker-compose file exists
		var clusterType string
		_, err = os.Stat(dockerComposeFilePath)
		if err != nil {
			return fmt.Errorf("Failed to generate join command: %s", err)
		} else {
			clusterType = string(onboard.ClusterType_VM)
		}

		command := fmt.Sprintf("knoxctl onboard node --type=%s --cp-addr=%s", clusterType, nodeAddr)
		fmt.Println(command)

		return nil
	},
}

func init() {
	onboardCmd.DisableFlagParsing = true
	onboardCmd.AddCommand(getJoinCmd)
}
