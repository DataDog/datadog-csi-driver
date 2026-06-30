// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"log/slog"
	"os"

	"github.com/spf13/afero"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
)

var remountReadOnlyBind = func(hostPath, targetPath string) error {
	return unix.Mount(hostPath, targetPath, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, "")
}

type bindMountArgs struct {
	hostPath   string
	targetPath string
	isFile     bool
	readOnly   bool
}

// bindMount performs a bind mount from hostPath to targetPath.
// It creates the target path if it doesn't exist (as file if isFile, directory otherwise).
// Returns nil if already mounted or mount succeeds.
func bindMount(afs afero.Afero, mounter mount.Interface, args bindMountArgs) error {
	slog.Info("bindMount: mounting", "host_path", args.hostPath, "target_path", args.targetPath, "read_only", args.readOnly)

	// Verify source path exists before attempting mount
	exists, err := afs.Exists(args.hostPath)
	if err != nil {
		return status.Errorf(codes.Internal, "bindMount: failed to check if source path exists: %v", err)
	}
	if !exists {
		return status.Errorf(codes.FailedPrecondition, "bindMount: source path %q does not exist", args.hostPath)
	}

	// Create target path if needed
	if err := createHostPath(afs, args.targetPath, args.isFile); err != nil {
		return err
	}

	// Check if already mounted
	// Note: IsLikelyNotMountPoint uses os.Stat internally, so we ignore "not exist" errors
	// which can happen in tests with MemMapFs or when the target was just created
	notMnt, err := mounter.IsLikelyNotMountPoint(args.targetPath)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "bindMount: failed to check mount point: %v", err)
	}

	// Perform bind mount if not already mounted
	if notMnt {
		options := []string{"bind"}
		if args.readOnly {
			options = append(options, "ro")
		}
		if err := mounter.Mount(args.hostPath, args.targetPath, "", options); err != nil {
			slog.Error("bindMount: failed to mount", "error", err, "host_path", args.hostPath, "target_path", args.targetPath)
			return status.Errorf(codes.Internal, "bindMount: failed to mount: %v", err)
		}
	} else {
		slog.Info("bindMount: already mounted, skipping", "target_path", args.targetPath)
		if args.readOnly {
			if err := remountReadOnlyBind(args.hostPath, args.targetPath); err != nil {
				slog.Error("bindMount: failed to remount read-only", "error", err, "host_path", args.hostPath, "target_path", args.targetPath)
				return status.Errorf(codes.Internal, "bindMount: failed to remount read-only: %v", err)
			}
		}
	}

	slog.Info("bindMount: successfully mounted", "host_path", args.hostPath, "target_path", args.targetPath, "read_only", args.readOnly)
	return nil
}

// bindUnmount unmounts the target path and removes it.
// Returns nil if target doesn't exist or unmount succeeds.
func bindUnmount(afs afero.Afero, mounter mount.Interface, targetPath string) error {
	slog.Info("bindUnmount: unmounting", "target_path", targetPath)

	// Check if target exists
	exists, err := afs.Exists(targetPath)
	if err != nil {
		return status.Errorf(codes.Internal, "bindUnmount: failed to check if target exists: %v", err)
	}
	if !exists {
		slog.Info("bindUnmount: target path does not exist, nothing to unmount", "target_path", targetPath)
		return nil
	}

	// Always attempt to unmount - IsLikelyNotMountPoint is unreliable for bind mounts
	// (See https://github.com/kubernetes/utils/blob/914a6e7505707ae6d13abe19730c24b4cfde9e6f/mount/mount.go#L59-L60)
	if err := mounter.Unmount(targetPath); err != nil {
		slog.Error("bindUnmount: failed to unmount", "error", err, "target_path", targetPath)
	}

	// Try to remove the target path
	if err := afs.RemoveAll(targetPath); err != nil {
		slog.Info("bindUnmount: failed to remove target path, Kubernetes will clean it up", "error", err, "target_path", targetPath)
	}

	slog.Info("bindUnmount: successfully unmounted", "target_path", targetPath)
	return nil
}
