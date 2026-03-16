package identity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaterialStatus records how a configured node-identity path resolves to local
// persisted runtime state for bootstrap inspection.
type MaterialStatus struct {
	ConfiguredPath string
	ResolvedPath   string
	Exists         bool
}

func inspectMaterial(dataDir, configuredPath, label string) (MaterialStatus, error) {
	status := MaterialStatus{
		ConfiguredPath: configuredPath,
		ResolvedPath:   resolveMaterialPath(dataDir, configuredPath),
	}

	info, err := os.Stat(status.ResolvedPath)
	switch {
	case err == nil:
		if info.IsDir() {
			return status, fmt.Errorf("%s %q must point to a file, not a directory", label, status.ResolvedPath)
		}
		status.Exists = true
		return status, nil
	case errors.Is(err, os.ErrNotExist):
		return status, nil
	default:
		return status, fmt.Errorf("stat %s %q: %w", label, status.ResolvedPath, err)
	}
}

func resolveMaterialPath(dataDir, configuredPath string) string {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return ""
	}
	if filepath.IsAbs(configuredPath) {
		return filepath.Clean(configuredPath)
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return filepath.Clean(configuredPath)
	}

	return filepath.Clean(filepath.Join(dataDir, configuredPath))
}

func (m MaterialStatus) Presence() string {
	if m.Exists {
		return "present"
	}
	return "missing"
}

func (m MaterialStatus) DisplayPath() string {
	if m.ConfiguredPath == "" || filepath.Clean(m.ConfiguredPath) == m.ResolvedPath {
		return m.ResolvedPath
	}
	return fmt.Sprintf("%s (from %q)", m.ResolvedPath, m.ConfiguredPath)
}
