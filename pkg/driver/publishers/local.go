package publishers

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

type localPublisher struct {
	fs      afero.Afero
	mounter mount.Interface
}

// Mount implements Publisher#Mount.
// It mounts directory hostPath onto directory targetPath.
// If hostPath is not found or is not a directory, it returns an error.
func (s localPublisher) Mount(targetPath string, hostPath string) error {
	// Check if the target path exists. Create if not present.
	if err := createHostPath(s.fs, targetPath, false); err != nil {
		return fmt.Errorf("failed to create required path %q: %w", targetPath, err)
	}

	notMnt, err := s.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "Error checking mount point: %v", err)
	}

	if notMnt {
		if err := s.mounter.Mount(hostPath, targetPath, "", []string{"bind"}); err != nil {
			klog.Errorf("Failed to mount %q to %q: %v", hostPath, targetPath, err)
			return status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}
	}

	return nil
}

func newLocalPublisher(fs afero.Afero, mounter mount.Interface) Publisher {
	return localPublisher{fs: fs, mounter: mounter}
}
