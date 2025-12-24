// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"os"
	"strings"

	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

// bindMount performs a bind mount from hostPath to targetPath.
// It creates the target path if it doesn't exist (as file if isFile, directory otherwise).
// Returns nil if already mounted or mount succeeds.
func bindMount(afs afero.Afero, mounter mount.Interface, hostPath, targetPath string, isFile bool) error {
	klog.Infof("bindMount: mounting %q to %q (isFile=%t)", hostPath, targetPath, isFile)

	// Verify source path exists before attempting mount
	exists, err := afs.Exists(hostPath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to check if source path exists: %v", err)
	}
	if !exists {
		return status.Errorf(codes.FailedPrecondition, "source path %q does not exist", hostPath)
	}

	// Create target path if needed
	if err := createHostPath(afs, targetPath, isFile); err != nil {
		return err
	}

	// Always attempt bind mount - IsLikelyNotMountPoint is unreliable for bind mounts
	// If already mounted, Mount will return an error that we can ignore
	if err := mounter.Mount(hostPath, targetPath, "", []string{"bind"}); err != nil {
		// Check if the error is because it's already mounted
		if isMountAlreadyExists(err) {
			klog.Infof("bindMount: %q is already mounted, skipping", targetPath)
			return nil
		}
		klog.Errorf("failed to mount %q to %q: %v", hostPath, targetPath, err)
		return status.Errorf(codes.Internal, "failed to mount: %v", err)
	}

	klog.Infof("bindMount: successfully mounted %q to %q", hostPath, targetPath)
	return nil
}

// isMountAlreadyExists checks if the error indicates the mount point is already mounted
func isMountAlreadyExists(err error) bool {
	// mount returns "already mounted" or similar when trying to mount to an existing mount point
	return err != nil && (strings.Contains(err.Error(), "already mounted") ||
		strings.Contains(err.Error(), "busy"))
}

// bindUnmount unmounts the target path and removes it.
// Returns nil if target doesn't exist or unmount succeeds.
func bindUnmount(mounter mount.Interface, targetPath string) error {
	// Check if target exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		klog.Infof("target path %q does not exist, nothing to unmount", targetPath)
		return nil
	}

	// Always attempt to unmount - IsLikelyNotMountPoint is unreliable for bind mounts
	// Unmount will fail gracefully if it's not a mount point
	if err := mounter.Unmount(targetPath); err != nil {
		// Log but don't fail - it might not be a mount point
		klog.V(4).Infof("unmount %q returned error (may not be a mount point): %v", targetPath, err)
	}

	// Remove the target path
	if err := os.RemoveAll(targetPath); err != nil {
		return status.Errorf(codes.Internal, "failed to remove target path %q: %v", targetPath, err)
	}

	return nil
}
