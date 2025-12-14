package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/podman/v6/pkg/machine/vmconfigs"
)

func TestCollectUnsharedHostPaths(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	shared := filepath.Join(tmp, "shared")
	nested := filepath.Join(shared, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	unshared := filepath.Join(tmp, "unshared")

	mounts := []*vmconfigs.Mount{
		{Source: shared},
	}

	volumes := []string{
		shared + ":/data",
		nested + ":/nested",
		unshared + ":/fail",
		unshared + ":/fail2", // duplicate should only be reported once
		"namedVolume:/ctr",
	}

	missing := collectUnsharedHostPaths(volumes, mounts)
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing mount, got %d (%v)", len(missing), missing)
	}
	if filepath.Clean(missing[0]) != filepath.Clean(unshared) {
		t.Fatalf("expected missing path %q, got %q", unshared, missing[0])
	}
}

func TestIsPathSharedWithMachine(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	shared := filepath.Join(tmp, "shared")
	if err := os.MkdirAll(filepath.Join(shared, "child"), 0o755); err != nil {
		t.Fatalf("mkdir shared child: %v", err)
	}
	mounts := []*vmconfigs.Mount{{Source: shared}}

	tests := []struct {
		path     string
		expected bool
	}{
		{filepath.Join(shared, "child"), true},
		{filepath.Join(shared, "child", "nested"), true},
		{filepath.Join(tmp, "elsewhere"), false},
	}

	for _, tc := range tests {
		if got := isPathSharedWithMachine(tc.path, mounts); got != tc.expected {
			t.Fatalf("unexpected result for %q: got %v, want %v", tc.path, got, tc.expected)
		}
	}
}
