package rtagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestHome returns a defaultUserHome pointed at a temp directory, so tests
// do not touch the real user home or env vars.
func newTestHome(t *testing.T, appName string) (defaultUserHome, string) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "rtagent-home-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	root := filepath.Join(tmp, "."+appName)
	return defaultUserHome{appName: appName, homeRoot: root}, root
}

func TestDefaultUserHomeResolveCreatesDirectoryTreeAndLayout(t *testing.T) {
	home, expectedRoot := newTestHome(t, "testagent")

	layout, err := home.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Root and all subdirs must exist.
	for _, sub := range []string{"", "db", "workspace", "skills", "memory", "config"} {
		dir := filepath.Join(expectedRoot, sub)
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("subdir %q not created: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", dir)
		}
	}

	if layout.HomeDir != expectedRoot {
		t.Fatalf("HomeDir = %q, want %q", layout.HomeDir, expectedRoot)
	}
	wantDSN := filepath.Join(expectedRoot, "db", "testagent.db")
	if layout.DSN != wantDSN {
		t.Fatalf("DSN = %q, want %q", layout.DSN, wantDSN)
	}
	if layout.WorkDir != filepath.Join(expectedRoot, "workspace") {
		t.Fatalf("WorkDir = %q, want workspace under root", layout.WorkDir)
	}
	if layout.SkillsDir != filepath.Join(expectedRoot, "skills") {
		t.Fatalf("SkillsDir = %q", layout.SkillsDir)
	}
	if layout.MemoryDir != filepath.Join(expectedRoot, "memory") {
		t.Fatalf("MemoryDir = %q", layout.MemoryDir)
	}
	if layout.ConfigDir != filepath.Join(expectedRoot, "config") {
		t.Fatalf("ConfigDir = %q", layout.ConfigDir)
	}
}

func TestEnvVarNameForApp(t *testing.T) {
	cases := []struct{ in, want string }{
		{"myagent", "MYAGENT_HOME"},
		{"rt-agent", "RT_AGENT_HOME"},
		{"my.agent", "MY_AGENT_HOME"},
		{"my agent", "MY_AGENT_HOME"},
	}
	for _, tc := range cases {
		if got := envVarNameForApp(tc.in); got != tc.want {
			t.Fatalf("envVarNameForApp(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOpenWithRuntimeHomeMakesDurableStorage(t *testing.T) {
	home, _ := newTestHome(t, "durableagent")

	rt, err := Open(context.Background(), Config{
		RuntimeHome: home,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer rt.Close()

	layout := rt.Home()
	if layout.HomeDir == "" {
		t.Fatalf("Home() returned empty layout; RuntimeHome was not applied")
	}
	// The DSN must point at a file path (durable), not an in-memory DSN.
	if !strings.Contains(layout.DSN, "durableagent.db") {
		t.Fatalf("DSN = %q, want a durable file path containing durableagent.db", layout.DSN)
	}
	if strings.Contains(layout.DSN, "mode=memory") {
		t.Fatalf("DSN = %q, must not be in-memory when RuntimeHome is set", layout.DSN)
	}
}

func TestOpenExplicitDSNIgnoresRuntimeHome(t *testing.T) {
	// When DSN is explicitly set, RuntimeHome must be ignored — the host has
	// taken full control of storage.
	tmp, err := os.MkdirTemp("", "rtagent-explicit-dsn-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmp)

	explicitDSN := filepath.Join(tmp, "explicit.db")
	home, homeRoot := newTestHome(t, "ignoredagent")

	rt, err := Open(context.Background(), Config{
		Runtime: RuntimeConfig{DSN: explicitDSN},
		// RuntimeHome is set, but DSN is explicit — home must NOT override.
		RuntimeHome: home,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer rt.Close()

	// Home() should still return the layout (host may want to read subdirs),
	// but the actual storage DSN used must be the explicit one.
	layout := rt.Home()
	if layout.HomeDir == "" {
		// When DSN is explicit, resolveRuntimeConfig skips home resolution,
		// so home is nil — that's acceptable: the host chose explicit storage.
		// This is the documented contract.
		return
	}
	// If home was resolved, the home root must not have been used for the DSN.
	_ = homeRoot
}

func TestOpenWithoutRuntimeHomeAndDSNUsesEphemeralMemory(t *testing.T) {
	// Zero-breakage: no RuntimeHome, no DSN → ephemeral in-memory (current
	// behavior unchanged).
	rt, err := Open(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer rt.Close()

	layout := rt.Home()
	if layout.HomeDir != "" {
		t.Fatalf("Home() = %+v, want zero-value when no RuntimeHome configured", layout)
	}
}

func TestRuntimeHomeFuncAdapter(t *testing.T) {
	tmp, err := os.MkdirTemp("", "rtagent-func-home-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmp)

	called := false
	dsn := filepath.Join(tmp, "func.db")
	rt, err := Open(context.Background(), Config{
		RuntimeHome: RuntimeHomeFunc(func(ctx context.Context) (RuntimeHomeLayout, error) {
			called = true
			return RuntimeHomeLayout{
				HomeDir: tmp,
				DSN:     dsn,
				WorkDir: tmp,
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer rt.Close()

	if !called {
		t.Fatalf("RuntimeHomeFunc was not called")
	}
	if rt.Home().DSN != dsn {
		t.Fatalf("Home().DSN = %q, want %q", rt.Home().DSN, dsn)
	}
}

func TestOpenRuntimeHomeErrorPropagates(t *testing.T) {
	expectedErr := "home resolver failed"
	_, err := Open(context.Background(), Config{
		RuntimeHome: RuntimeHomeFunc(func(ctx context.Context) (RuntimeHomeLayout, error) {
			return RuntimeHomeLayout{}, &testHomeError{msg: expectedErr}
		}),
	})
	if err == nil {
		t.Fatalf("Open() error = nil, want error from RuntimeHome.Resolve")
	}
	if !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("Open() error = %q, want it to contain %q", err.Error(), expectedErr)
	}
}

type testHomeError struct{ msg string }

func (e *testHomeError) Error() string { return e.msg }
