//go:build !amd64 && !arm64

package common

func WarnIfMachineVolumesUnavailable(volumeSpecs []string) {}
