// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
)

// bindMount performs a bind mount from hostPath to targetPath.
// It creates the target path if it doesn't exist (as file if isFile, directory otherwise).
// Returns nil if already mounted or mount succeeds.
func bindMount(afs afero.Afero, mounter mount.Interface, hostPath, targetPath string, isFile bool) error {
	// Create target path if needed
	if err := createHostPath(afs, targetPath, isFile); err != nil {
		return err
	}

	// Check if already mounted
	notMnt, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "error checking mount point: %v", err)
	}

	// Perform bind mount if not already mounted
	if notMnt {
		if err := mounter.Mount(hostPath, targetPath, "", []string{"bind"}); err != nil {
			log.Error().Err(err).Str("host_path", hostPath).Str("target_path", targetPath).Msg("failed to mount")
			return status.Errorf(codes.Internal, "failed to mount: %v", err)
		}
	}

	return nil
}

// bindUnmount unmounts the target path and removes it.
// Returns nil if target doesn't exist or unmount succeeds.
func bindUnmount(mounter mount.Interface, targetPath string) error {
	// Check if the target path is a mount point
	isNotMnt, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Target doesn't exist, nothing to unmount
			log.Info().Str("target_path", targetPath).Msg("target path does not exist, nothing to unmount")
			return nil
		}
		return status.Errorf(codes.Internal, "failed to check if target path is a mount point: %v", err)
	}

	// If it's a mount point, unmount it
	if !isNotMnt {
		if err := mounter.Unmount(targetPath); err != nil {
			log.Error().Err(err).Str("target_path", targetPath).Msg("failed to unmount target path")
			return status.Errorf(codes.Internal, "failed to unmount target path %q: %v", targetPath, err)
		}
	} else {
		log.Info().Str("target_path", targetPath).Msg("target path is not a mount point, skipping unmount")
	}

	// Remove the target path
	if err := os.RemoveAll(targetPath); err != nil {
		return status.Errorf(codes.Internal, "failed to remove target path %q: %v", targetPath, err)
	}

	return nil
}
