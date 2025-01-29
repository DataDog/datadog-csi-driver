package utils

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureSocketAvailability(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		mockRemoveFunc func(string) error
		expectError    bool
		expectedAddr   string
	}{
		{
			name:        "Invalid scheme",
			endpoint:    "http://localhost:8080",
			expectError: true,
		},
		{
			name:        "Invalid URL",
			endpoint:    "unix:*@msksocket/csi.sock",
			expectError: true,
		},
		{
			name:         "Valid Unix endpoint, no existing socket",
			endpoint:     "unix:///tmp/my_socket.sock",
			expectError:  false,
			expectedAddr: "/tmp/my_socket.sock",
		},
		{
			name:         "Valid Unix endpoint, no existing socket",
			endpoint:     "unix:///tmp/my_socket.sock",
			expectError:  false,
			expectedAddr: "/tmp/my_socket.sock",
		},
		{
			name:         "Valid Unix endpoint with host",
			endpoint:     "unix://localhost/tmp/my_socket.sock",
			expectError:  false,
			expectedAddr: "localhost/tmp/my_socket.sock",
		},
		{
			name:           "Failure to remove existing socket",
			endpoint:       "unix://localhost/tmp/my_socket.sock",
			expectError:    true,
			mockRemoveFunc: func(string) error { return errors.New("missing permissions") },
		},
		{
			name:           "Valid socket, socket doesn't already exist",
			endpoint:       "unix:///tmp/my_socket.sock",
			expectError:    false,
			mockRemoveFunc: func(string) error { return os.ErrNotExist },
			expectedAddr:   "/tmp/my_socket.sock",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Mock os.Remove function
			if test.mockRemoveFunc != nil {
				removeFile = test.mockRemoveFunc
			} else {
				// Reset to default behavior if no mock provided
				removeFile = os.Remove
			}

			address, err := EnsureSocketAvailability(test.endpoint)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedAddr, address)
		})
	}
}
