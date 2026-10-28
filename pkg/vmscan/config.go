package vmscan

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/accuknox/omniscan/pkg/publisher/cspm"
)

const (
	DefaultUploadTimeout = 15 * time.Minute
	InstallerURL         = "https://accuknox-omni.s3.us-east-1.amazonaws.com/latest/agent-install.sh"
)

type Config struct {
	Endpoint      string
	Token         string
	TenantID      uint
	Label         string
	ClusterID     *int
	ClusterName   string
	RootFS        string
	DBDirectory   string
	Hostname      string
	SkipDBUpdate  bool
	Offline       bool
	OutputDir     string
	ScriptURL     string
	ScriptPath    string
	BinaryURL     string
	InstallDir    string
	UploadTimeout time.Duration
}

func (c Config) Validate() error {
	u, err := url.ParseRequestURI(c.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("artifact endpoint must be a full HTTPS URL")
	}
	if strings.TrimSpace(c.Token) == "" || c.TenantID == 0 || strings.TrimSpace(c.Label) == "" {
		return fmt.Errorf("artifact token, a positive tenant-id, and artifact label are required")
	}
	if err := (cspm.Config{Endpoint: c.Endpoint, Token: c.Token, TenantID: c.TenantID, LabelID: c.Label}).Validate(); err != nil {
		return fmt.Errorf("invalid artifact API token: provide the raw artifact JWT without a Bearer prefix or surrounding quotes (join tokens and license keys are separate credentials): %w", err)
	}
	return nil
}
