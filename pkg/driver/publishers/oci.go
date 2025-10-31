// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package publishers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Datadog/datadog-csi-driver/pkg/env"
	"github.com/Datadog/datadog-csi-driver/pkg/oci"
	"github.com/Datadog/datadog-csi-driver/pkg/paths"
	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

type ociPublisher struct {
	fs      afero.Afero
	mounter mount.Interface
}

// Mount implements Publisher#Mount.
// It mounts directory hostPath onto directory targetPath.
// If hostPath is not found or is not a directory, it returns an error.
func (s ociPublisher) Mount(targetPath string, hostPath string, volumeContext map[string]string) error {
	pkg := volumeContext["package"]
	ver := volumeContext["version"]

	pullable := fmt.Sprintf("oci://install.datadoghq.com/%s-package:%s", strings.TrimPrefix(pkg, "datadog-"), ver)

	dir, err := os.MkdirTemp("", "example-*")
	if err != nil {
		return fmt.Errorf("could not create directory")
	}

	// Check if the target path exists. Create if not present.
	if err := createHostPath(s.fs, targetPath, false); err != nil {
		return fmt.Errorf("failed to create required path %q: %w", targetPath, err)
	}

	downloader := oci.NewDownloader(env.FromEnv(), http.DefaultClient)
	p, err := downloader.Download(context.Background(), pullable)
	if err != nil {
		return fmt.Errorf("could not download image: %w", err)
	}
	err = p.ExtractLayers(oci.DatadogPackageLayerMediaType, dir)
	if err != nil {
		return fmt.Errorf("could not extract layers: %w", err)
	}

	err = os.MkdirAll(paths.PackagesPath, 0755)
	if err != nil {
		return fmt.Errorf("could make packages path: %w", err)
	}

	hostPath, err = movePackageFromSource(pkg, paths.PackagesPath, dir)
	if err != nil {
		return fmt.Errorf("could not move package: %w", err)
	}

	// // Check if the target path exists. Create if not present.
	// if err := createHostPath(s.fs, targetPath, false); err != nil {
	// 	return fmt.Errorf("failed to create required path %q: %w", targetPath, err)
	// }

	notMnt, err := s.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "Error checking mount point: %v", err)
	}

	// if true {
	// 	return fmt.Errorf("hostPath: %s, targetPath: %s", hostPath, targetPath)
	// }

	if notMnt {
		if err := s.mounter.Mount(hostPath, targetPath, "", []string{"bind"}); err != nil {
			klog.Errorf("Failed to mount %q to %q: %v", hostPath, targetPath, err)
			return status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}
	}

	return nil
}

func newOCIPublisher(fs afero.Afero, mounter mount.Interface) Publisher {
	return ociPublisher{fs: fs, mounter: mounter}
}

func movePackageFromSource(packageName string, rootPath string, sourcePath string) (string, error) {
	if packageName == "" {
		return "", fmt.Errorf("invalid package name")
	}
	targetPath := filepath.Join(rootPath, packageName)
	_, err := os.Stat(targetPath)
	//if err == nil {
	//	return "", fmt.Errorf("target package already exists")
	//}
	if err == nil {
		return targetPath, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("could not stat target package: %w", err)
	}
	if err := paths.SetRepositoryPermissions(sourcePath); err != nil {
		return "", fmt.Errorf("could not set permissions on package: %w", err)
	}
	err = os.Rename(sourcePath, targetPath)
	if err != nil {
		return "", fmt.Errorf("could not move source: %w", err)
	}
	return targetPath, nil
}
