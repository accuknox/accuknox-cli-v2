package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/accuknox/accuknox-cli-v2/pkg/privilege"
	"github.com/spf13/cobra"
)

// Every command uses the same permission-error handler. Existing root sessions,
// dry runs, and non-permission failures never trigger another sudo execution.
func handlePrivilegeError(cmd *cobra.Command, err error, uid int, elevate func(*cobra.Command) error) error {
	if err == nil || uid == 0 || runtime.GOOS == "windows" || cmd == nil || commandDryRun(cmd) || !privilege.IsPermissionError(err) {
		return err
	}
	return elevate(cmd)
}

func commandDryRun(cmd *cobra.Command) bool {
	if flag := cmd.Flag("dry-run"); flag != nil {
		return flag.Value.String() == "true"
	}
	return false
}

func flagTrue(cmd *cobra.Command, name string) bool {
	flag := cmd.Flag(name)
	return flag != nil && flag.Value.String() == "true"
}

func elevateCommand(cmd *cobra.Command) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	args := append([]string{}, os.Args[1:]...)
	// Keep a mode inferred by the original process when it differs from the
	// flag's default, since root may have different Docker access.
	if flag := cmd.Flag("vm-mode"); flag != nil && !flag.Changed && flag.Value.String() != flag.DefValue {
		args = append(args, "--vm-mode="+flag.Value.String())
	}
	return privilege.Run(cmd.Context(), executable, args, os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:               privilege.BootstrapCommand,
		Hidden:            true,
		Args:              cobra.NoArgs,
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() != 0 {
				return fmt.Errorf("the internal elevated entry point requires root")
			}
			request, err := privilege.ReadRequest(os.Stdin)
			if err != nil {
				return err
			}
			if request.Arguments[0] == privilege.BootstrapCommand {
				return fmt.Errorf("recursive elevation request")
			}
			if err := request.RestoreEnvironment(); err != nil {
				return err
			}
			if request.TerminalInput {
				terminal, err := os.Open("/dev/tty")
				if err != nil {
					return fmt.Errorf("restoring interactive terminal input: %w", err)
				}
				original, originalInput := os.Stdin, rootCmd.InOrStdin()
				os.Stdin = terminal
				rootCmd.SetIn(terminal)
				defer func() {
					os.Stdin = original
					rootCmd.SetIn(originalInput)
					terminal.Close()
				}()
			}
			rootCmd.SetArgs(request.Arguments)
			return rootCmd.ExecuteContext(cmd.Context())
		},
	})
}
