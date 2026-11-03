package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var sysdumpVMCmd = newVMSysdumpCmd()

func newVMSysdumpCmd() *cobra.Command {
	var mode, configPath, outputDir string
	var logTail int
	var timeout time.Duration
	c := &cobra.Command{
		Use:   "sysdump",
		Short: "Collect VM agent diagnostics from systemd or Docker deployments",
		Args:  cobra.NoArgs,
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			if mode != "" && mode != string(onboard.VMMode_Systemd) && mode != string(onboard.VMMode_Docker) {
				return fmt.Errorf("invalid vm-mode %q: expected docker or systemd", mode)
			}
			if logTail < 0 || timeout <= 0 {
				return fmt.Errorf("log-tail must be nonnegative and timeout must be positive")
			}
			selectedMode := mode
			if selectedMode == "" {
				services, err := onboard.CheckInstalledSystemdServices()
				if err != nil {
					return fmt.Errorf("checking systemd installation: %w", err)
				}
				selectedMode = string(onboard.VMMode_Docker)
				if len(services) > 0 {
					selectedMode = string(onboard.VMMode_Systemd)
				}
			}
			hostname, err := os.Hostname()
			if err != nil {
				hostname = "unknown-host"
			}
			workRoot := ""
			archiveRoot := "."
			if outputDir != "" {
				archiveRoot, err = filepath.Abs(outputDir)
				if err != nil {
					return fmt.Errorf("resolving output directory: %w", err)
				}
				if err := os.MkdirAll(archiveRoot, 0700); err != nil {
					return fmt.Errorf("creating output directory: %w", err)
				}
				workRoot = archiveRoot
			}
			tmpDir, err := os.MkdirTemp(workRoot, "knoxctl-sysdump-"+hostname+"-")
			if err != nil && outputDir == "" {
				tmpDir, err = os.MkdirTemp(".", "knoxctl-sysdump-"+hostname+"-")
			}
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Collecting %s VM diagnostics in %s\n", selectedMode, tmpDir)
			switch selectedMode {
			case string(onboard.VMMode_Systemd):
				fmt.Fprintln(cmd.OutOrStdout(), "Copying agent installation and knoxctl dumps...")
				onboard.DumpSystemdAgentInstallation(tmpDir)
				onboard.DumpSystemdKnoxctlDir(tmpDir)
			case string(onboard.VMMode_Docker):
				ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
				warnings, err := onboard.DumpDockerDiagnostics(ctx, tmpDir, configPath, logTail)
				cancel()
				if err != nil {
					return err
				}
				for _, warning := range warnings {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", warning)
				}
				if len(warnings) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "Some diagnostics could not be collected; see docker/errors.log in the archive.")
				}
			}
			archive, err := filepath.Abs(filepath.Join(archiveRoot, filepath.Base(tmpDir)+".tar.gz"))
			if err != nil {
				return err
			}
			if err := archiveVMSysdump(tmpDir, archive); err != nil {
				return fmt.Errorf("creating sysdump archive: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created %s\n", archive)
			return nil
		},
	}
	c.Flags().StringVar(&mode, "vm-mode", "", "installation mode: systemd or docker (automatically detected if omitted)")
	c.Flags().StringVarP(&outputDir, "output-dir", "o", "", "directory for both working files and the final archive (default: temporary workspace and archive in current directory)")
	c.Flags().StringVar(&configPath, "config-path", "", "Docker onboarding config directory (automatically detected from agent mounts if omitted)")
	c.Flags().IntVar(&logTail, "log-tail", 10000, "maximum log lines per Docker container; 0 collects all available logs")
	c.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "maximum Docker diagnostic collection time")
	return c
}

func archiveVMSysdump(sourceDir, filename string) (result error) {
	output, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(output)
	tw := tar.NewWriter(gz)
	defer func() {
		result = errors.Join(result, tw.Close(), gz.Close(), output.Close())
		if result != nil {
			os.Remove(filename)
		}
	}()
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(filepath.Dir(sourceDir), path)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(relative)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(tw, file, info.Size())
			return errors.Join(copyErr, file.Close())
		}
		return nil
	})
}

func init() {
	vmCmd.AddCommand(sysdumpVMCmd)
}
