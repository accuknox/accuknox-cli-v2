package onboard

import (
	"fmt"
	"os"
	"path/filepath"
)

// Docker mounts configPath at /opt. Prepare the outbox for the image's UID 1000
// so it survives container recreation without running sumengine as root.
func prepareSumengineMessageStore(configPath string, enabled bool) error {
	if !enabled {
		return nil
	}
	parent := filepath.Join(configPath, "accuknox-sumengine")
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("creating sumengine data directory: %w", err)
	}
	path := filepath.Join(parent, "message-store")
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("creating sumengine message-store: %w", err)
	}
	if err := os.Chown(path, 1000, 1000); err != nil {
		return fmt.Errorf("setting sumengine message-store owner: %w", err)
	}
	if err := os.Chmod(path, 0700); err != nil {
		return fmt.Errorf("setting sumengine message-store permissions: %w", err)
	}
	return nil
}
