package fs

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// LocalVss is a wrapper around the local file system which uses windows volume
// shadow copy service (VSS) in a transparent way.
type LocalVss struct {
	wrappedFs FS
	snapshots map[string]VssSnapshot
	lock      *sync.Mutex
}

// statically ensure that LocalVss implements FS.
var _ FS = &LocalVss{}

// NewLocalVss creates a new wrapper around the windows filesystem using volume
// shadow copy service to access locked files.
func NewLocalVss() LocalVss {
	return LocalVss{wrappedFs: Local{}, snapshots: make(map[string]VssSnapshot), lock: &sync.Mutex{}}
}

// DeleteSnapshots deletes all snapshots that were created automatically.
func (fs LocalVss) DeleteSnapshots() {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	fmt.Printf("LocalVss: DeleteSnapshots()")

	for _, snapshot := range fs.snapshots {
		if err := snapshot.Delete(); err != nil {
			fmt.Printf("LocalVss: Failed to delete snapshot: %s", err)
		}
	}
}

// VolumeName returns leading volume name. Given "C:\foo\bar" it returns "C:"
// on Windows. Given "\\host\share\foo" it returns "\\host\share". On other
// platforms it returns "".
func (fs LocalVss) VolumeName(path string) string {
	return fs.wrappedFs.VolumeName(path)
}

// Open opens a file for reading.
func (fs LocalVss) Open(name string) (File, error) {
	return fs.wrappedFs.Open(fs.snapshotPath(name))
}

// OpenFile is the generalized open call; most users will use Open
// or Create instead.  It opens the named file with specified flag
// (O_RDONLY etc.) and perm, (0666 etc.) if applicable.  If successful,
// methods on the returned File can be used for I/O.
// If there is an error, it will be of type *PathError.
func (fs LocalVss) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return fs.wrappedFs.OpenFile(fs.snapshotPath(name), flag, perm)
}

// Stat returns a FileInfo describing the named file. If there is an error, it
// will be of type *PathError.
func (fs LocalVss) Stat(name string) (os.FileInfo, error) {
	return fs.wrappedFs.Stat(fs.snapshotPath(name))
}

// Lstat returns the FileInfo structure describing the named file.
// If the file is a symbolic link, the returned FileInfo
// describes the symbolic link.  Lstat makes no attempt to follow the link.
// If there is an error, it will be of type *PathError.
func (fs LocalVss) Lstat(name string) (os.FileInfo, error) {
	return fs.wrappedFs.Lstat(fs.snapshotPath(name))
}

// Join joins any number of path elements into a single path, adding a
// Separator if necessary. Join calls Clean on the result; in particular, all
// empty strings are ignored. On Windows, the result is a UNC path if and only
// if the first path element is a UNC path.
func (fs LocalVss) Join(elem ...string) string {
	return fs.wrappedFs.Join(elem...)
}

// Separator returns the OS and FS dependent separator for dirs/subdirs/files.
func (fs LocalVss) Separator() string {
	return fs.wrappedFs.Separator()
}

// IsAbs reports whether the path is absolute.
func (fs LocalVss) IsAbs(path string) bool {
	return fs.wrappedFs.IsAbs(path)
}

// Abs returns an absolute representation of path. If the path is not absolute
// it will be joined with the current working directory to turn it into an
// absolute path. The absolute path name for a given file is not guaranteed to
// be unique. Abs calls Clean on the result.
func (fs LocalVss) Abs(path string) (string, error) {
	return fs.wrappedFs.Abs(path)
}

// Clean returns the cleaned path. For details, see filepath.Clean.
func (fs LocalVss) Clean(p string) string {
	return fs.wrappedFs.Clean(p)
}

// Base returns the last element of path.
func (fs LocalVss) Base(path string) string {
	return fs.wrappedFs.Base(path)
}

// Dir returns path without the last element.
func (fs LocalVss) Dir(path string) string {
	return fs.wrappedFs.Dir(path)
}

// snapshotPath returns the path inside a VSS snapshots if it already exists.
// If the path is not yet available as a snapshot a snapshot is created.
// If creation of a snapshot fails the file's original path is returned.
func (fs LocalVss) snapshotPath(path string) string {
	fixPath := strings.TrimPrefix(fixpath(path), `\\?\`)
	volumeName := fixPath[:2]

	fs.lock.Lock()
	defer fs.lock.Unlock()

	// ensure snapshot for volume exists
	if _, ok := fs.snapshots[volumeName]; !ok {
		vssVolume := volumeName + `\`
		fmt.Printf("GetSnapshotPath: Creating snapshot for [%s]\r\n", vssVolume)

		if snapshot, err := NewVssSnapshot(vssVolume, 120); err != nil {
			// TODO: error handling
			fmt.Printf("GetSnapshotPath: Failed to create snapshot for [%s]: %s\r\n", volumeName, err)
		} else {
			fs.snapshots[volumeName] = snapshot
			fmt.Printf("GetSnapshotPath: Successfully created snapshot for [%s]\r\n", vssVolume)
		}
	}

	if snapshot, ok := fs.snapshots[volumeName]; !ok {
		// TODO: log warning
		return path
	} else {
		vssPath := fs.Join(snapshot.GetSnapshotDeviceObject(), strings.TrimPrefix(fixPath, volumeName))
		return vssPath
	}
}
