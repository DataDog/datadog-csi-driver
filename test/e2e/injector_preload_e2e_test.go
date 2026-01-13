//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

func TestInjectorPreloadPublisher(t *testing.T) {
	ctx := context.TODO()
	clientset := setupClient(t)

	podNames := []string{"consumer-injector-preload"}

	if !waitForPodsRunning(t, ctx, clientset, podNames, 60*time.Second) {
		t.FailNow()
	}

	t.Log("Verifying injector preload file contents...")
	assertPodLogsContain(t, ctx, clientset, "consumer-injector-preload", []string{
		"Injector preload test passed",
	}, 30*time.Second)
}
