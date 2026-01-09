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
)

func createHostPath(fs afero.Afero, pathname string, isFile bool) error {
	log.Info().Str("path", pathname).Bool("is_file", isFile).Msg("Checking if path exists")

	// Check if the pathname exists
	exists, err := fs.Exists(pathname)
	if err != nil {
		log.Error().Err(err).Str("path", pathname).Msg("Error checking path")
		return status.Errorf(codes.Internal, "Error checking path: %v", err)
	}
	if !exists {
		if isFile {
			log.Info().Str("path", pathname).Msg("File does not exist, creating")
			// Create the file
			file, err := fs.Create(pathname)
			if err != nil {
				log.Error().Err(err).Str("path", pathname).Msg("Failed to create file")
				return status.Errorf(codes.Internal, "Cannot create file: %v", err)
			}
			defer file.Close() // Ensure the file gets closed after creation
			log.Info().Str("path", pathname).Msg("Successfully created file")
		} else {
			log.Info().Str("path", pathname).Msg("Directory does not exist, creating")
			const dirPerm = 0755
			// Create the directory
			if err := fs.MkdirAll(pathname, dirPerm); err != nil {
				log.Error().Err(err).Str("path", pathname).Msg("Failed to create directory")
				return status.Errorf(codes.Internal, "Cannot create directory: %v", err)
			}
			// Set permissions explicitly
			if err := fs.Chmod(pathname, dirPerm); err != nil {
				log.Error().Err(err).Str("path", pathname).Msg("Failed to set permissions for directory")
				return status.Errorf(codes.Internal, "Cannot set permissions: %v", err)
			}
			log.Info().Str("path", pathname).Msg("Successfully created and set permissions for directory")
		}
	} else {
		log.Info().Str("path", pathname).Msg("Path already exists")
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
