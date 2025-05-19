//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestUDSConsumerPod(t *testing.T) {
	ctx := context.TODO()

	// Setup client
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		t.Fatalf("Failed to load kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create clientset: %v", err)
	}

	consumers := []struct {
		podName     string
		expectedLog string
	}{
		{
			podName:     "consumer-dsdsocket",
			expectedLog: "Hello from DSDSocket",
		},
		{
			podName:     "consumer-dsdsocketdirectory",
			expectedLog: "Hello from DSDSocketDirectory",
		},
		{
			podName:     "consumer-apmsocket",
			expectedLog: "Hello from APMSocket",
		},
		{
			podName:     "consumer-apmsocketdirectory",
			expectedLog: "Hello from APMSocketDirectory",
		},
		{
			podName:     "deprecated-consumer-dsdsocket",
			expectedLog: "Hello from deprecated DSDSocket",
		},
		{
			podName:     "deprecated-consumer-dsdsocketdirectory",
			expectedLog: "Hello from deprecated DSDSocketDirectory",
		},
		{
			podName:     "deprecated-consumer-apmsocket",
			expectedLog: "Hello from deprecated APMSocket",
		},
		{
			podName:     "deprecated-consumer-apmsocketdirectory",
			expectedLog: "Hello from deprecated APMSocketDirectory",
		},
	}

	t.Log("Waiting for consumer pods to be Ready...")
	assert.EventuallyWithTf(t, func(collect *assert.CollectT) {
		for _, consumer := range consumers {
			// Fetch the pod
			pod, err := clientset.CoreV1().Pods("default").Get(ctx, consumer.podName, metav1.GetOptions{})
			if err != nil {
				collect.Errorf("Error fetching pod %v: %v", consumer.podName, err)
				return
			}

			// Check if the pod is running
			if pod.Status.Phase != corev1.PodRunning {
				collect.Errorf("Pod %v is not running, current phase: %v", consumer.podName, pod.Status.Phase)
			}
		}
	}, 60*time.Second, 2*time.Second, "uds-consumer pod did not reach Running state")

	t.Log("Verifying successful communication over UDS...")
	assert.EventuallyWithTf(t, func(collect *assert.CollectT) {
		for _, consumer := range consumers {
			// Fetch logs from the dummy-agent pod (this will show messages received from the socket)
			var dummyAgentLogs []byte
			dummyAgentLogs, err = clientset.CoreV1().Pods("default").GetLogs("dummy-agent", &corev1.PodLogOptions{}).DoRaw(ctx)
			if err != nil {
				t.Fatalf("Failed to fetch dummy-agent logs: %v", err)
			}

			// Assert that the message from uds-consumer is found in dummy-agent logs
			assert.Contains(t, string(dummyAgentLogs), consumer.expectedLog, fmt.Sprintf("dummy-agent should receive the message from %v", consumer.podName))
		}
	}, 60*time.Second, 2*time.Second, "uds-consumer pod did not reach Running state")

}
