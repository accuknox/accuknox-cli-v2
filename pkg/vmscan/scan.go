package vmscan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/accuknox/omniscan/pkg/job"
	"github.com/accuknox/omniscan/pkg/job/agent"
	"github.com/accuknox/omniscan/pkg/publisher"
	"github.com/accuknox/omniscan/pkg/publisher/cspm"
	"github.com/accuknox/omniscan/pkg/publisher/file"
	"github.com/accuknox/omniscan/pkg/util"
	"github.com/aquasecurity/trivy/pkg/db"
	fanaltypes "github.com/aquasecurity/trivy/pkg/fanal/types"
	"github.com/aquasecurity/trivy/pkg/javadb"
	"github.com/google/go-containerregistry/pkg/name"
)

// Scan runs Omni's VM vulnerability scanner and optional malware scanning, without container scanning.
func Scan(ctx context.Context, c Config, out io.Writer) error {
	return ScanWithProgress(ctx, c, out, os.Stderr)
}

// ScanWithProgress writes timestamped progress to logs while keeping the JSON
// summary on out. Long operations report their current phase every 30 seconds.
func ScanWithProgress(ctx context.Context, c Config, out, logs io.Writer) (scanErr error) {
	progress := newScanProgress(ctx, logs, 30*time.Second)
	defer func() { progress.finish(ctx, scanErr) }()
	progress.stage(ctx, "validating scanner configuration")
	if c.UploadTimeout < 0 {
		return fmt.Errorf("upload timeout must be positive")
	}
	if c.UploadTimeout == 0 {
		c.UploadTimeout = DefaultUploadTimeout
	}
	var pub publisher.Publisher
	var err error
	if c.OutputDir != "" {
		pub = file.NewPublisher(c.OutputDir)
	} else {
		if err := c.Validate(); err != nil {
			return err
		}
		pub, err = cspm.NewPublisher(cspm.Config{Endpoint: c.Endpoint, Token: c.Token, TenantID: c.TenantID, LabelID: c.Label}, progress.logger.With("publisher", "cspm"))
		if err != nil {
			return err
		}
	}
	pub = observedPublisher{next: pub, progress: progress, timeout: c.UploadTimeout, remote: c.OutputDir == ""}
	if c.MalwareScan {
		progress.stage(ctx, "checking malware scanner prerequisites")
		checkMalwarePrerequisites(&c, logs)
	}
	progress.stage(ctx, "detecting host filesystem")
	if c.RootFS == "" {
		c.RootFS, err = util.FindRootFS()
		if err != nil {
			return fmt.Errorf("determining root filesystem: %w", err)
		}
	}
	if util.DetectOS(c.RootFS) == "unknown" {
		return fmt.Errorf("cannot determine OS at %q", c.RootFS)
	}
	if c.DBDirectory == "" {
		c.DBDirectory = "/var/lib/accuknox-omni"
	}
	if c.Offline {
		if util.DetectOS(c.RootFS) != "linux" {
			return fmt.Errorf("offline VM scanning currently supports Linux root filesystems only")
		}
		if err := validateLocalDatabases(c.DBDirectory); err != nil {
			return err
		}
		c.SkipDBUpdate = true
	}
	if util.DetectOS(c.RootFS) == "linux" {
		progress.stage(ctx, "checking vulnerability database")
		client := db.NewClient(db.Dir(c.DBDirectory), true)
		update, err := client.NeedsUpdate(ctx, "knoxctl", c.SkipDBUpdate)
		if err != nil {
			return fmt.Errorf("checking vulnerability database: %w", err)
		}
		if update {
			progress.stage(ctx, "preparing vulnerability database")
			if err := client.Download(ctx, filepath.Join(c.DBDirectory, "db"), fanaltypes.RegistryOptions{}); err != nil {
				return fmt.Errorf("downloading vulnerability database: %w", err)
			}
		}
		progress.stage(ctx, "vulnerability database ready")
		var javaRepositories []name.Reference
		for _, repository := range []string{javadb.DefaultGCRRepository, javadb.DefaultGHCRRepository} {
			ref, err := name.ParseReference(repository)
			if err != nil {
				return err
			}
			javaRepositories = append(javaRepositories, ref)
		}
		progress.stage(ctx, "preparing Java database")
		javadb.Init(c.DBDirectory, javaRepositories, c.SkipDBUpdate, true, fanaltypes.RegistryOptions{})
		if err := javadb.Update(); err != nil {
			return fmt.Errorf("preparing Java vulnerability database: %w", err)
		}
		progress.stage(ctx, "Java vulnerability database ready")
	}
	progress.stage(ctx, "creating VM vulnerability scan job")
	opts := []agent.OptFunc{
		agent.WithLogger(progress.logger),
		agent.WithContainerVulnScanDisabled(),
		agent.WithVulnDBDirectory(c.DBDirectory),
		agent.WithOverrideHostname(c.Hostname),
		agent.WithClusterName(c.ClusterName),
	}
	if !c.MalwareScan {
		opts = append(opts, agent.WithMalwareScanDisabled())
	} else {
		opts = append(opts, agent.WithClamAVDBDirectory(c.malwareDBDirectory()), agent.WithMalwareScanTargetDirectory(c.RootFS))
	}
	if c.ClusterID != nil {
		opts = append(opts, agent.WithClusterID(*c.ClusterID))
	}
	j, err := agent.NewJob(pub, opts...)
	if err != nil {
		return fmt.Errorf("creating VM scan job: %w", err)
	}
	progress.stage(ctx, "scanning host packages")
	summary, err := j.Run(ctx, c.RootFS)
	if err != nil {
		return fmt.Errorf("running VM scan: %w", err)
	}
	progress.stage(ctx, "writing scan summary")
	if err := json.NewEncoder(out).Encode(summary); err != nil {
		return err
	}
	return summaryError(summary)
}

func summaryError(summary job.ErrorSummaryMap) error {
	var failures []error
	for host, result := range summary {
		if err := errors.Join(result.OrchestrationError, result.VulnScanError, result.MalwareScanError, result.ContainerVulnScanError, result.ArtifactAPIError); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", host, err))
		}
	}
	return errors.Join(failures...)
}
