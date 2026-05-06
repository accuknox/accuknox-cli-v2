// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/cbom"
	"github.com/spf13/cobra"
)

var cbomOpts cbom.Options

var cbomCmd = &cobra.Command{
	Use:   "cbom",
	Short: "Generate Cryptography Bill of Materials (CBOM)",
	Long: `Generate a CycloneDX-compliant Cryptography Bill of Materials (CBOM)
that inventories all cryptographic algorithms, protocols, and certificates
found in source code or a container image.`,
}

var cbomSourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Generate CBOM from source code",
	Long: `Scan a source code directory for cryptographic assets (algorithms,
protocols, certificates). Produces a CycloneDX 1.6 CBOM.

Example:
  knoxctl cbom source --path ./myapp
  knoxctl cbom source --path ./myapp --format table
  knoxctl cbom source --path ./myapp --out cbom.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bom, err := cbom.GenerateFromSource(&cbomOpts)
		if err != nil {
			return err
		}
		fmt.Printf("Found %d cryptographic component(s)\n", cbom.ComponentCount(bom))
		return cbom.Output(bom, &cbomOpts)
	},
}

var cbomImageCmd = &cobra.Command{
	Use:   "image",
	Short: "Generate CBOM from a container image",
	Long: `Scan a container image for cryptographic assets (certificates, keys,
TLS configuration, secrets). Produces a CycloneDX CBOM.

Example:
  knoxctl cbom image --image nginx:latest
  knoxctl cbom image --image registry.io/myapp:v1.2 --format table
  knoxctl cbom image --image nginx:latest --out cbom.json
  knoxctl cbom image --image nginx:latest --plugins certificates,keys`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bom, err := cbom.GenerateFromImage(&cbomOpts)
		if err != nil {
			return err
		}
		fmt.Printf("Found %d cryptographic component(s)\n", cbom.ComponentCount(bom))
		return cbom.Output(bom, &cbomOpts)
	},
}

func init() {
	rootCmd.AddCommand(cbomCmd)
	cbomCmd.AddCommand(cbomSourceCmd)
	cbomCmd.AddCommand(cbomImageCmd)

	// source flags
	cbomSourceCmd.Flags().StringVar(&cbomOpts.Path, "path", ".", "Source directory to scan")

	// image flags
	cbomImageCmd.Flags().StringVar(&cbomOpts.Image, "image", "", "Container image reference (e.g. nginx:latest)")
	cbomImageCmd.Flags().StringVar(&cbomOpts.BOMFile, "bom", "", "Existing BOM file to enrich/verify")
	cbomImageCmd.Flags().StringVar(&cbomOpts.Plugins, "plugins", "", "Comma-separated plugin list (e.g. certificates,keys)")
	cbomImageCmd.Flags().StringVar(&cbomOpts.Ignore, "ignore", "", "Glob patterns to exclude from scanning")

	// common flags on the parent — inherited by both subcommands
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Name, "name", "", "Project name (defaults to path or image reference)")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Group, "group", "", "Project group or module prefix (e.g. com.example or github.com/org)")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Version, "version", "", "Project version (e.g. 1.2.3)")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Description, "description", "", "Short description of the project")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.License, "license", "", "SPDX license identifier (e.g. Apache-2.0)")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.OutputTo, "out", "", "Write CBOM JSON to this file")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Format, "format", "json", `Output format: "json" or "table"`)

	// Signing flags (only meaningful when --out is set)
	cbomCmd.PersistentFlags().BoolVar(&cbomOpts.Sign.Enabled, "sign", false, "Sign the output artifact with cosign after generation")
	cbomCmd.PersistentFlags().BoolVar(&cbomOpts.Sign.GenerateKey, "sign-generate-key", false, "Generate a new ECDSA P-256 key pair before signing")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Sign.KeyRef, "sign-key", "", "Path to existing cosign private key (default: cosign.key)")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Sign.KeyOut, "sign-key-out", "cosign", "Filename prefix for generated key pair (produces <prefix>.key / <prefix>.pub)")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Sign.Password, "sign-key-password", "", "Passphrase for the signing key (empty = no passphrase)")
	cbomCmd.PersistentFlags().StringVar(&cbomOpts.Sign.SigOut, "sign-sig-out", "", "Path to write the signature (default: <out>.sig)")
}
