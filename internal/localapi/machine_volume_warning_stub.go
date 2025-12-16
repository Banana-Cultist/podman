//go:build !amd64 && !arm64

package localapi

func WarnIfMachineVolumesUnavailable(_ bool, _ string, _ []string) {}
