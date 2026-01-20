//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUDSConsumerPod(t *testing.T) {
	ctx := context.TODO()
	clientset := setupClient(t)

	consumers := []struct {
		podName     string
		expectedLog string
	}{
		{"consumer-dsdsocket", "Hello from DSDSocket"},
		{"consumer-dsdsocketdirectory", "Hello from DSDSocketDirectory"},
		{"consumer-apmsocket", "Hello from APMSocket"},
		{"consumer-apmsocketdirectory", "Hello from APMSocketDirectory"},
		{"deprecated-consumer-dsdsocket", "Hello from deprecated DSDSocket"},
		{"deprecated-consumer-dsdsocketdirectory", "Hello from deprecated DSDSocketDirectory"},
		{"deprecated-consumer-apmsocket", "Hello from deprecated APMSocket"},
		{"deprecated-consumer-apmsocketdirectory", "Hello from deprecated APMSocketDirectory"},
		{"deprecated-consumer-datadogsocketsdirectory", "Hello from deprecated DatadogSocketsDirectory"},
	}

	// Extract pod names for waitForPodsRunning
	podNames := make([]string, len(consumers))
	for i, c := range consumers {
		podNames[i] = c.podName
	}

	if !waitForPodsRunning(t, ctx, clientset, podNames, 60*time.Second) {
		t.FailNow()
	}

	t.Log("Verifying successful communication over UDS...")
	dummyAgentLogs := getPodLogs(t, ctx, clientset, "dummy-agent")

	for _, consumer := range consumers {
		assert.True(t,
			strings.Contains(dummyAgentLogs, consumer.expectedLog),
			"dummy-agent should receive the message from %s", consumer.podName,
		)
	}
}
