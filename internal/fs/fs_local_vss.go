package fs

import (
	"os"
	"strings"
	"sync"
)

// MessageHandler is used to report errors/messages via callbacks.
type MessageHandler func(msg string, args ...interface{})

// LocalVss is a wrapper around the local file system which uses windows volume
// shadow copy service (VSS) in a transparent way.
type LocalVss struct {
	FS
	snapshots  map[string]VssSnapshot
	lock       *sync.Mutex
	msgError   MessageHandler
	msgMessage MessageHandler
	msgVerbose MessageHandler
}

// statically ensure that LocalVss implements FS.
var _ FS = &LocalVss{}

// NewLocalVss creates a new wrapper around the windows filesystem using volume
// shadow copy service to access locked files.
func NewLocalVss(msgError, msgMessage, msgVerbose MessageHandler) LocalVss {
	return LocalVss{
		FS:         Local{},
		snapshots:  make(map[string]VssSnapshot),
		lock:       &sync.Mutex{},
		msgError:   msgError,
		msgMessage: msgMessage,
		msgVerbose: msgVerbose,
	}
}

// DeleteSnapshots deletes all snapshots that were created automatically.
func (fs LocalVss) DeleteSnapshots() {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	for _, snapshot := range fs.snapshots {
		if err := snapshot.Delete(); err != nil {
			fs.msgError("failed to delete VSS snapshot: %s", err)
		}
	}
}

// Open  wraps the Open method of the underlying file system.
func (fs LocalVss) Open(name string) (File, error) {
	return fs.FS.Open(fs.snapshotPath(name))
}

// OpenFile wraps the Open method of the underlying file system.
func (fs LocalVss) OpenFile(
	name string, flag int, perm os.FileMode,
) (File, error) {
	return fs.FS.OpenFile(fs.snapshotPath(name), flag, perm)
}

// Stat wraps the Open method of the underlying file system.
func (fs LocalVss) Stat(name string) (os.FileInfo, error) {
	return fs.FS.Stat(fs.snapshotPath(name))
}

// Lstat wraps the Open method of the underlying file system.
func (fs LocalVss) Lstat(name string) (os.FileInfo, error) {
	return fs.FS.Lstat(fs.snapshotPath(name))
}

// snapshotPath returns the path inside a VSS snapshots if it already exists.
// If the path is not yet available as a snapshot, a snapshot is created.
// If creation of a snapshot fails the file's original path is returned as
// a fallback.
func (fs LocalVss) snapshotPath(path string) string {
	fixPath := strings.TrimPrefix(fixpath(path), `\\?\`)
	volumeName := fixPath[:2]

	fs.lock.Lock()

	// ensure snapshot for volume exists
	if _, ok := fs.snapshots[volumeName]; !ok {
		vssVolume := volumeName + `\`
		fs.msgMessage("creating VSS snapshot for [%s]\r\n", vssVolume)

		if snapshot, err := NewVssSnapshot(vssVolume, 120); err != nil {
			fs.msgError(
				"failed to create snapshot for [%s]: %s\r\n",
				volumeName, err,
			)
		} else {
			fs.snapshots[volumeName] = snapshot
			fs.msgVerbose(
				"successfully created snapshot for [%s]\r\n",
				vssVolume,
			)
		}
	}

	var snapshotPath string
	if snapshot, ok := fs.snapshots[volumeName]; ok {
		snapshotPath = fs.Join(
			snapshot.GetSnapshotDeviceObject(),
			strings.TrimPrefix(fixPath, volumeName))
	} else {
		// TODO: log warning?
		snapshotPath = path
	}

	fs.lock.Unlock()
	return snapshotPath
}
