// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	log "log/slog"
	"os"

	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func createHostPath(fs afero.Afero, pathname string, isFile bool) error {
	log.Info("Checking if path exists", "path", pathname, "is_file", isFile)

	// Check if the pathname exists
	exists, err := fs.Exists(pathname)
	if err != nil {
		log.Error("Error checking path", "error", err, "path", pathname)
		return status.Errorf(codes.Internal, "Error checking path: %v", err)
	}
	if !exists {
		if isFile {
			log.Info("File does not exist, creating", "path", pathname)
			// Create the file
			file, err := fs.Create(pathname)
			if err != nil {
				log.Error("Failed to create file", "error", err, "path", pathname)
				return status.Errorf(codes.Internal, "Cannot create file: %v", err)
			}
			defer file.Close() // Ensure the file gets closed after creation
			log.Info("Successfully created file", "path", pathname)
		} else {
			log.Info("Directory does not exist, creating", "path", pathname)
			const dirPerm = 0755
			// Create the directory
			if err := fs.MkdirAll(pathname, dirPerm); err != nil {
				log.Error("Failed to create directory", "error", err, "path", pathname)
				return status.Errorf(codes.Internal, "Cannot create directory: %v", err)
			}
			// Set permissions explicitly
			if err := fs.Chmod(pathname, dirPerm); err != nil {
				log.Error("Failed to set permissions for directory", "error", err, "path", pathname)
				return status.Errorf(codes.Internal, "Cannot set permissions: %v", err)
			}
			log.Info("Successfully created and set permissions for directory", "path", pathname)
		}
	} else {
		log.Info("Path already exists", "path", pathname)
	}

	return nil
}

// isSocketPath checks if a file is a socket.
func isSocketPath(fs afero.Afero, path string) (bool, error) {
	fileInfo, err := fs.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.Mode().Type() == os.ModeSocket, nil
}
