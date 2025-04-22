// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package utils

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// This is defined only for mocking purposes
var removeFile func(string) error = os.Remove

// EnsureSocketAvailability extracts unix address from a unix endpoint
// and ensures it is available, deleting the socket if it already exists.
// If existing socket can't be deleted, or the passed endpoint is not a unix
// endpoint, an error is returned.
func EnsureSocketAvailability(unixEndpoint string) (address string, err error) {
	parsedURL, err := url.Parse(unixEndpoint)
	if err != nil {
		return "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	if len(parsedURL.Path) == 0 {
		return "", errors.New("endpoint path can't be empty")
	}

	if len(parsedURL.Host) == 0 {
		address = filepath.FromSlash(parsedURL.Path)
	} else {
		address = path.Join(parsedURL.Host, filepath.FromSlash(parsedURL.Path))
	}

	if scheme := strings.ToLower(parsedURL.Scheme); scheme != "unix" {
		return "", fmt.Errorf("%q is not a unix endpoint", unixEndpoint)
	}

	if err := removeFile(address); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("could not remove unix socket %q: %v", address, err)
	}

	return address, nil
}
