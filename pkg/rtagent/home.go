package rtagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultUserHome is a ready-to-use RuntimeHome that resolves a persistent
// home directory under the user's home directory (or an environment override).
// It creates the directory tree on Resolve so a zero-config Open becomes
// durable. Hosts can use this directly or implement RuntimeHome themselves for
// any convention (custom location, permissions, layout, config-file format).
type defaultUserHome struct {
	appName string
	// homeRoot overrides the resolved root directory; when empty, the root is
	// computed from the env var or the user home dir. Used by tests to point
	// at a temp directory without setting env vars.
	homeRoot string
}

// DefaultUserHome returns a RuntimeHome that resolves ~/.<appName> (or
// $<UPPER_APP_NAME>_HOME when set) and creates the standard subdirectory
// layout: db/, workspace/, skills/, memory/, config/. The SQLite DSN points at
// db/<appName>.db. Use this to make zero-config Open durable.
func DefaultUserHome(appName string) RuntimeHome {
	return defaultUserHome{appName: appName}
}

func (h defaultUserHome) Resolve(ctx context.Context) (RuntimeHomeLayout, error) {
	root := strings.TrimSpace(h.homeRoot)
	if root == "" {
		root = h.resolveRootFromEnvOrHome()
	}
	if root == "" {
		return RuntimeHomeLayout{}, fmt.Errorf("could not resolve runtime home directory for %q", h.appName)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return RuntimeHomeLayout{}, fmt.Errorf("resolve runtime home path: %w", err)
	}

	// Create the directory tree. This is the host-chosen implementation doing
	// the creation, not the kernel — the kernel only consumes the resulting
	// DSN/WorkDir.
	subdirs := []string{"db", "workspace", "skills", "memory", "config"}
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o700); err != nil {
			return RuntimeHomeLayout{}, fmt.Errorf("create runtime home subdir %q: %w", sub, err)
		}
	}

	return RuntimeHomeLayout{
		HomeDir:   root,
		DSN:       filepath.Join(root, "db", h.appName+".db"),
		WorkDir:   filepath.Join(root, "workspace"),
		SkillsDir: filepath.Join(root, "skills"),
		MemoryDir: filepath.Join(root, "memory"),
		ConfigDir: filepath.Join(root, "config"),
	}, nil
}

// resolveRootFromEnvOrHome returns $<UPPER_APP_NAME>_HOME when set, otherwise
// ~/.<appName>. Returns "" if neither the env var nor the user home dir is
// available.
func (h defaultUserHome) resolveRootFromEnvOrHome() string {
	envName := envVarNameForApp(h.appName)
	if env := strings.TrimSpace(os.Getenv(envName)); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, "."+h.appName)
}

// envVarNameForApp converts an app name like "myagent" to the env var
// "MYAGENT_HOME".
func envVarNameForApp(appName string) string {
	upper := strings.ToUpper(strings.TrimSpace(appName))
	upper = strings.ReplaceAll(upper, "-", "_")
	upper = strings.ReplaceAll(upper, ".", "_")
	upper = strings.ReplaceAll(upper, " ", "_")
	return upper + "_HOME"
}
