//go:build amd64 || arm64

package localapi

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	libpodDefine "go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgenutilexternal"
)

const machineVolumesDocURL = "https://docs.podman.io/en/latest/markdown/podman-machine-init.1.html#volume"

// WarnIfMachineVolumesUnavailable inspects bind mounts requested via --volume
// and --mount and warns if the source paths are not shared with the active
// Podman machine.
func WarnIfMachineVolumesUnavailable(cfg *entities.PodmanConfig, volumeSpecs, mountSpecs []string) {
	if cfg == nil || (len(volumeSpecs) == 0 && len(mountSpecs) == 0) || !cfg.MachineMode {
		return
	}

	parsedURI, err := url.Parse(cfg.URI)
	if err != nil {
		logrus.Debugf("skipping machine volume check, invalid connection URI %q: %v", cfg.URI, err)
		return
	}

	mounts, vmType, err := getMachineMountsAndVMType(cfg.URI, parsedURI)
	if err != nil {
		logrus.Debugf("skipping machine volume check: %v", err)
		return
	}
	if vmType == define.WSLVirt {
		// WSL mounts the drives automatically so a warning would be misleading.
		return
	}

	missing := collectUnsharedHostPaths(volumeSpecs, mountSpecs, mounts, vmType)
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	logrus.Warnf("The following bind mount sources are not shared with the Podman machine and may not work: %s. See %s for details on configuring machine volumes.", strings.Join(missing, ", "), machineVolumesDocURL)
}

func collectUnsharedHostPaths(volumeSpecs, mountSpecs []string, mounts []*vmconfigs.Mount, vmType define.VMType) []string {
	unshared := []string{}
	seen := make(map[string]struct{})
	record := func(src string) {
		if _, found := IsPathAvailableOnMachine(mounts, vmType, src); found {
			return
		}
		normalized, err := normalizeVolumeSource(src)
		if err != nil {
			logrus.Debugf("machine volume check: unable to normalize %q: %v", src, err)
			return
		}
		if _, exists := seen[normalized]; !exists {
			unshared = append(unshared, normalized)
			seen[normalized] = struct{}{}
		}
	}

	for _, spec := range volumeSpecs {
		if src, ok := extractBindMountSource(spec); ok {
			record(src)
		}
	}
	for _, spec := range mountSpecs {
		if src, ok := extractMountFlagSource(spec); ok {
			record(src)
		}
	}
	return unshared
}

func extractBindMountSource(spec string) (string, bool) {
	parts := specgen.SplitVolumeString(spec)
	if len(parts) <= 1 {
		return "", false
	}
	return resolveHostPathSource(parts[0])
}

func extractMountFlagSource(spec string) (string, bool) {
	mountType, tokens, err := specgenutilexternal.FindMountType(spec)
	if err != nil {
		logrus.Debugf("machine volume check: unable to parse mount %q: %v", spec, err)
		return "", false
	}
	if mountType != libpodDefine.TypeBind && mountType != "glob" {
		return "", false
	}

	var src, dest string
	for _, token := range tokens {
		name, value, hasValue := strings.Cut(token, "=")
		if !hasValue {
			continue
		}
		switch name {
		case "src", "source":
			src = value
		case "target", "dst", "dest", "destination":
			dest = value
		}
	}
	// A bind mount without an explicit source uses the destination as the host
	// path, mirroring getBindMount() in pkg/specgenutil.
	if src == "" && mountType == libpodDefine.TypeBind {
		src = dest
	}
	return resolveHostPathSource(src)
}

func resolveHostPathSource(src string) (string, bool) {
	if len(src) == 0 {
		return "", false
	}
	if strings.HasPrefix(src, "./") {
		resolved, err := filepath.EvalSymlinks(src)
		if err != nil {
			logrus.Debugf("machine volume check: failed to resolve symlinks of %q: %v", src, err)
		} else {
			path, err := filepath.Abs(resolved)
			if err != nil {
				logrus.Debugf("machine volume check: failed to get absolute path of %q: %v", resolved, err)
			} else {
				src = path
			}
		}
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
