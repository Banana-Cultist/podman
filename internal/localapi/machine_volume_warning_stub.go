//go:build !amd64 && !arm64

package localapi

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func WarnIfMachineVolumesUnavailable(_ *entities.PodmanConfig, _, _ []string) {
	logrus.Debug("skipping machine volume check: podman machine mode not supported")
}
