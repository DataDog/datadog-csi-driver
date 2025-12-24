// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
)

// bindMount performs a bind mount from hostPath to targetPath.
// It creates the target path if it doesn't exist (as file if isFile, directory otherwise).
// Returns nil if already mounted or mount succeeds.
func bindMount(afs afero.Afero, mounter mount.Interface, hostPath, targetPath string, isFile bool) error {
	// Verify source path exists before attempting mount
	exists, err := afs.Exists(hostPath)
	if err != nil {
		return status.Errorf(codes.Internal, "bindMount: failed to check if source path exists: %v", err)
	}
	if !exists {
		return status.Errorf(codes.FailedPrecondition, "bindMount: source path %q does not exist", hostPath)
	}

	// Create target path if needed
	if err := createHostPath(afs, targetPath, isFile); err != nil {
		return err
	}

	// Always attempt bind mount - IsLikelyNotMountPoint is unreliable for bind mounts
	// (See https://github.com/kubernetes/utils/blob/914a6e7505707ae6d13abe19730c24b4cfde9e6f/mount/mount.go#L59-L60)
	//
	// If already mounted, Mount will return an error that we can ignore
	if err := mounter.Mount(hostPath, targetPath, "", []string{"bind"}); err != nil {
		// Check if the error is because it's already mounted
		if isMountAlreadyExists(err) {
			slog.Info("bindMount: already mounted, skipping", "target_path", targetPath)
			return nil
		}
		slog.Error("bindMount: failed to mount", "error", err, "host_path", hostPath, "target_path", targetPath)
		return status.Errorf(codes.Internal, "bindMount: failed to mount: %v", err)
	}

	slog.Info("bindMount: successfully mounted", "host_path", hostPath, "target_path", targetPath)
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
		slog.Info("bindUnmount: target path does not exist, nothing to unmount", "target_path", targetPath)
		return nil
	}

	// Always attempt to unmount - IsLikelyNotMountPoint is unreliable for bind mounts (see comment in bindMount)
	if err := mounter.Unmount(targetPath); err != nil {
		slog.Error("bindUnmount: failed to unmount", "error", err, "target_path", targetPath)
		return status.Errorf(codes.Internal, "bindUnmount: failed to unmount %q: %v", targetPath, err)
	}

	// Remove the target path
	if err := os.RemoveAll(targetPath); err != nil {
		return status.Errorf(codes.Internal, "bindUnmount: failed to remove target path %q: %v", targetPath, err)
	}

	slog.Info("bindUnmount: successfully unmounted", "target_path", targetPath)
	return nil
}
