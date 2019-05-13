// +build !windows

package fs

import (
	"github.com/restic/restic/internal/errors"
)

type VssSnapshot struct{}

func NewVssSnapshot(volume string, timeoutInSeconds uint) (VssSnapshot, error) {
	return VssSnapshot{}, errors.New("VSS snapshots are only supported on windows")
}

func (p *VssSnapshot) Delete() error {
	return nil
}

func (p *VssSnapshot) GetSnapshotDeviceObject() string {
	return ""
}
