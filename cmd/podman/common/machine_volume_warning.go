//go:build amd64 || arm64

package common

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/containers/podman/v6/cmd/podman/registry"
	"github.com/containers/podman/v6/internal/localapi"
	"github.com/containers/podman/v6/pkg/machine/define"
	"github.com/containers/podman/v6/pkg/machine/vmconfigs"
	"github.com/containers/podman/v6/pkg/specgen"
	"github.com/sirupsen/logrus"
)

const machineVolumesDocURL = "https://docs.podman.io/en/latest/markdown/podman-machine-init.1.html#volume"

// WarnIfMachineVolumesUnavailable inspects bind mounts requested via --volume
// and warns if the source paths are not shared with the active Podman machine.
func WarnIfMachineVolumesUnavailable(volumeSpecs []string) {
	if len(volumeSpecs) == 0 {
		return
	}

	cfg := registry.PodmanConfig()
	if cfg == nil || !cfg.MachineMode {
		return
	}

	connectionURI := cfg.URI
	if len(connectionURI) == 0 {
		return
	}

	parsedURI, err := url.Parse(connectionURI)
	if err != nil {
		logrus.Debugf("skipping machine volume check, invalid connection URI %q: %v", connectionURI, err)
		return
	}

	machineConfig, provider, err := localapi.FindMachineByPort(connectionURI, parsedURI)
	if err != nil {
		logrus.Debugf("skipping machine volume check: %v", err)
		return
	}
	if provider.VMType() == define.WSLVirt {
		// WSL mounts the drives automatically so a warning would be misleading.
		return
	}

	missing := collectUnsharedHostPaths(volumeSpecs, machineConfig.Mounts)
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	logrus.Warnf("The following bind mount sources are not shared with the Podman machine and may not work: %s. See %s for details on configuring machine volumes.", strings.Join(missing, ", "), machineVolumesDocURL)
}

func collectUnsharedHostPaths(volumeSpecs []string, mounts []*vmconfigs.Mount) []string {
	unshared := []string{}
	seen := make(map[string]struct{})
	for _, spec := range volumeSpecs {
		src, ok := extractBindMountSource(spec)
		if !ok {
			continue
		}
		normalized, err := normalizeVolumeSource(src)
		if err != nil {
			logrus.Debugf("machine volume check: unable to normalize %q: %v", src, err)
			continue
		}
		if !isPathSharedWithMachine(normalized, mounts) {
			if _, exists := seen[normalized]; !exists {
				unshared = append(unshared, normalized)
				seen[normalized] = struct{}{}
			}
		}
	}
	return unshared
}

func extractBindMountSource(spec string) (string, bool) {
	parts := specgen.SplitVolumeString(spec)
	if len(parts) <= 1 {
		return "", false
	}
	src := parts[0]
	if len(src) == 0 {
		return "", false
	}
	if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") || specgen.IsHostWinPath(src) {
		return src, true
	}
	return "", false
}

func normalizeVolumeSource(path string) (string, error) {
	if specgen.IsHostWinPath(path) {
		return filepath.Clean(path), nil
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return absPath, nil
}

func isPathSharedWithMachine(path string, mounts []*vmconfigs.Mount) bool {
	if len(mounts) == 0 {
		return false
	}
	cleanPath := filepath.Clean(path)
	for _, mount := range mounts {
		if mount == nil || len(mount.Source) == 0 {
			continue
		}
		source := filepath.Clean(mount.Source)
		rel, err := filepath.Rel(source, cleanPath)
		if err != nil {
			continue
		}
		rel = filepath.Clean(rel)
		if rel == "." {
			return true
		}
		if strings.HasPrefix(rel, "..") {
			continue
		}
		return true
	}
	return false
}
