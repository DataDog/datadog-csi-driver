// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

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

type socketPublisher struct {
	fs      afero.Afero
	mounter mount.Interface
}

// Mount implements Publisher#Mount.
// It verifies that hostPath is indeed a UDS socket path.
// If it is not the case, an error is returned.
// Else, it mounts the socket path onto targetPath
func (s socketPublisher) Mount(targetPath string, hostPath string) error {
	hostPathIsSocket, err := isSocketPath(hostPath)

	if err != nil {
		return fmt.Errorf("failed to check if %q is a socket path: %v", hostPath, err)
	}

	if !hostPathIsSocket {
		return fmt.Errorf("socket not found at %q", hostPath)
	}

	// Check if the target path exists. Create if not present.
	if err := createHostPath(s.fs, targetPath, true); err != nil {
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

func newSocketPublisher(fs afero.Afero, mounter mount.Interface) Publisher {
	return socketPublisher{fs: fs, mounter: mounter}
}
