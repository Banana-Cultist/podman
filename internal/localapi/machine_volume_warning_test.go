//go:build amd64 || arm64

package localapi

import (
	"os"
	"path/filepath"
	"testing"

	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
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

	missing := collectUnsharedHostPaths(volumes, nil, mounts, define.QemuVirt)
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing mount, got %d (%v)", len(missing), missing)
	}
	if filepath.Clean(missing[0]) != filepath.Clean(unshared) {
		t.Fatalf("expected missing path %q, got %q", unshared, missing[0])
	}
}

func TestCollectUnsharedHostPathsMountFlag(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	shared := filepath.Join(tmp, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatalf("mkdir shared: %v", err)
	}
	unshared := filepath.Join(tmp, "unshared")
	mounts := []*vmconfigs.Mount{{Source: shared}}

	for _, tc := range []struct {
		name  string
		mount string
		want  string
	}{
		{"bind unshared src", "type=bind,src=" + unshared + ",target=/data", unshared},
		{"bind unshared source", "type=bind,source=" + unshared + ",dst=/data", unshared},
		{"bind shared", "type=bind,src=" + shared + ",target=/data", ""},
		{"glob unshared", "type=glob,src=" + unshared + "/*,target=/data", unshared + "/*"},
		{"named volume", "type=volume,src=myvol,target=/data", ""},
		{"tmpfs", "type=tmpfs,target=/data", ""},
		// A bind mount with no source uses the destination as the host path.
		{"bind no source", "type=bind,target=" + unshared, unshared},
		// Defaults to a named volume when no type is given.
		{"no type", "src=myvol,target=/data", ""},
		{"unparsable", `type=bind,src="broken`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			missing := collectUnsharedHostPaths(nil, []string{tc.mount}, mounts, define.QemuVirt)
			if tc.want == "" {
				if len(missing) != 0 {
					t.Fatalf("expected no missing mounts, got %v", missing)
				}
				return
			}
			if len(missing) != 1 {
				t.Fatalf("expected 1 missing mount, got %d (%v)", len(missing), missing)
			}
			if filepath.Clean(missing[0]) != filepath.Clean(tc.want) {
				t.Fatalf("expected missing path %q, got %q", tc.want, missing[0])
			}
		})
	}
}

// A path named by both --volume and --mount is reported only once.
func TestCollectUnsharedHostPathsDedupesAcrossFlags(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	unshared := filepath.Join(tmp, "unshared")
	mounts := []*vmconfigs.Mount{{Source: filepath.Join(tmp, "shared")}}

	missing := collectUnsharedHostPaths(
		[]string{unshared + ":/data"},
		[]string{"type=bind,src=" + unshared + ",target=/other"},
		mounts, define.QemuVirt,
	)
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing mount, got %d (%v)", len(missing), missing)
	}
}
