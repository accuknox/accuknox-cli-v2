package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var sysdumpVMCmd = &cobra.Command{
	Use:   "sysdump",
	Short: "Command to get system debug info",
	Long:  "Command to get system debug info",
	RunE: func(cmd *cobra.Command, args []string) error {
		// find out the VM Mode
		var (
			installedSystemdServices = []string{"kubearmor", cm.KubeArmorVMAdapter, cm.RelayServer, cm.PEAAgent, cm.SIAAgent, cm.FeederService, cm.SpireAgent, cm.SummaryEngine, cm.DiscoverAgent, cm.HardeningAgent}
			err                      error
		)

		if vmMode == "" {
			installedSystemdServices, err = onboard.CheckInstalledSystemdServices()
			if err != nil {
				return fmt.Errorf(color.RedString("error checking systemd files"))
			}

			if len(installedSystemdServices) > 0 {
				vmMode = onboard.VMMode_Systemd
			} else {
				vmMode = onboard.VMMode_Docker
			}
		}

		hostname, err := os.Hostname()
		if err != nil {
			fmt.Println(color.YellowString("Failed to get hostname", err.Error()))
		}

		// create directory
		tmpDirRoot := "/tmp"
		sysdumpDir := fmt.Sprintf(fmt.Sprintf("knoxctl-sysdump-%s-", hostname))
		tmpDir, err := os.MkdirTemp(tmpDirRoot, sysdumpDir)
		if err != nil {
			tmpDirRoot = "."
			tmpDir, err = os.MkdirTemp(tmpDirRoot, sysdumpDir)
			if err != nil {
				return err
			}
		}

		fmt.Println(color.BlueString("Using temp dir %s", tmpDir))

		switch vmMode {
		case onboard.VMMode_Systemd:
			// get logs from all the installed systemd services of our interest
			// fmt.Println(color.GreenString("Copying logs..."))
			// onboard.DumpSystemdLogs(tmpDir, installedSystemdServices)

			fmt.Println(color.GreenString("Copying agent installation..."))
			onboard.DumpSystemdAgentInstallation(tmpDir)

			// additional data
			fmt.Println(color.GreenString("Copying knoxctl dumps..."))
			onboard.DumpSystemdKnoxctlDir(tmpDir)

		case onboard.VMMode_Docker:
			// TODO: sysdump for docker mode

		default:
			fmt.Printf(color.RedString("vm mode: %s invalid, accepted values (docker/systemd)", vmMode))
		}

		tarFileName := filepath.Base(tmpDir) + ".tar.gz"
		tarFile, err := os.OpenFile(filepath.Clean(tarFileName), os.O_CREATE|os.O_WRONLY, 0644) // #nosec G302 file permissions needed for archiving
		if err != nil {
			return err
		}

		gzipWriter := gzip.NewWriter(tarFile)
		tarWriter := tar.NewWriter(gzipWriter)

		// add logs
		err = filepath.WalkDir(tmpDir, func(path string, d fs.DirEntry, err error) error {
			if d == nil {
				return nil
			}

			fileInfo, err := d.Info()
			if err != nil {
				return err
			}

			relativePath, err := filepath.Rel(tmpDirRoot, path)
			if err != nil {
				return err
			}

			hdr, err := tar.FileInfoHeader(fileInfo, relativePath)
			if err != nil {
				return err
			}
			hdr.Name = relativePath

			if err := tarWriter.WriteHeader(hdr); err != nil {
				return err
			}

			if !d.IsDir() {
				fileContent, err := os.ReadFile(filepath.Clean(path))
				if err != nil {
					return err
				}

				if _, err := tarWriter.Write(fileContent); err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf(color.RedString("Failed to produce %s: %s"), tarFileName, err.Error())
		}

		if err := tarWriter.Close(); err != nil {
			return err
		}

		if err := gzipWriter.Close(); err != nil {
			return err
		}

		fmt.Println(color.BlueString("Removing temp dir %s...", tmpDir))
		err = os.RemoveAll(tmpDir)
		if err != nil {
			return err
		}

		fmt.Println(color.GreenString("Successfully created "))

		return nil
	},
}

func init() {
	sysdumpVMCmd.PersistentFlags().StringVar((*string)(&vmMode), "vm-mode", "", "Mode of installation (systemd/docker)")
	vmCmd.AddCommand(sysdumpVMCmd)
}
