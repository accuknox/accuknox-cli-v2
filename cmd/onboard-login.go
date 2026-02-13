package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	username string
	password string

	usernameSTDIN bool
	passwordSTDIN bool

	idToken      string
	idTokenSTDIN bool
)

// loginCmd represents the onboard non-k8s cluster command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Command for authenticating with OCI registries. Default - DockerHub",
	Long:  "Command for authenticating with OCI registries. Default - DockerHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		// if password empty then take from stdin by default
		if idTokenSTDIN {
			passwordSTDIN = true
			usernameSTDIN = false
		} else if idToken == "" {
			if password != "" {
				passwordSTDIN = false
			}

			if username != "" {
				usernameSTDIN = false
			}
		}

		loginOpts := onboard.LoginOptions{
			Registry:           registry,
			RegistryConfigPath: registryConfigPath,
			Username:           username,
			UserPass:           password,
			UsernameSTDIN:      usernameSTDIN,
			PasswordSTDIN:      passwordSTDIN,
			IDTokenSTDIN:       idTokenSTDIN,
			PlainHTTP:          plainHTTP,
			Insecure:           insecure,
		}

		err := loginOpts.ORASRegistryLogin()
		if err != nil {
			return err
		}

		fmt.Println("Login Successful")

		return nil
	},
}

func init() {

	loginCmd.PersistentFlags().StringVarP(&username, "username", "u", "", "username for authenticating")

	loginCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "password for authenticating")
	loginCmd.PersistentFlags().BoolVarP(&passwordSTDIN, "password-stdin", "s", true, "password from stdin")
	loginCmd.PersistentFlags().StringVarP(&idToken, "identity-token", "", "", "identity-token for authenticating")
	loginCmd.PersistentFlags().BoolVarP(&idTokenSTDIN, "identity-token-stdin", "", false, "identity-token from stdin")

	loginCmd.MarkFlagsMutuallyExclusive("password", "password-stdin", "identity-token", "identity-token-stdin")

	loginCmd.PersistentFlags().Lookup("password-stdin").NoOptDefVal = "true"
	loginCmd.PersistentFlags().Lookup("identity-token-stdin").NoOptDefVal = "true"

	onboardCmd.AddCommand(loginCmd)
}
