//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

func TestLibraryPublisher(t *testing.T) {
	ctx := context.TODO()
	clientset := setupClient(t)

	consumers := []struct {
		podName     string
		expectedLog string
	}{
		{"consumer-library", "Library test passed"},
		{"consumer-library-digest", "Library digest test passed"},
	}

	// Extract pod names for waitForPodsRunning
	podNames := make([]string, len(consumers))
	for i, c := range consumers {
		podNames[i] = c.podName
	}

	if !waitForPodsRunning(t, ctx, clientset, podNames, 120*time.Second) {
		t.FailNow()
	}

	t.Log("Verifying library contents...")
	for _, consumer := range consumers {
		assertPodLogsContain(t, ctx, clientset, consumer.podName, []string{
			consumer.expectedLog,
		}, 30*time.Second)
	}
}
