//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultNamespace = "default"
)

// setupClient creates a Kubernetes clientset from the default kubeconfig.
func setupClient(t *testing.T) *kubernetes.Clientset {
	t.Helper()

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	require.NoError(t, err, "Failed to load kubeconfig")

	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err, "Failed to create clientset")

	return clientset
}

// waitForPodRunning waits for a pod to reach the Running phase.
// Returns true if the pod is running, false otherwise.
// On failure, it logs the pod events for debugging.
func waitForPodRunning(t *testing.T, ctx context.Context, clientset *kubernetes.Clientset, podName string, timeout time.Duration) bool {
	t.Helper()

	t.Logf("Waiting for pod %s to be Running...", podName)

	success := assert.EventuallyWithTf(t, func(collect *assert.CollectT) {
		pod, err := clientset.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			collect.Errorf("Error fetching pod %s: %v", podName, err)
			return
		}

		if pod.Status.Phase != corev1.PodRunning {
			collect.Errorf("Pod %s is not running, current phase: %v", podName, pod.Status.Phase)
		}
	}, timeout, 2*time.Second, "pod %s did not reach Running state", podName)

	if !success {
		logPodEvents(t, ctx, clientset, podName)
	}

	return success
}

// waitForPodsRunning waits for multiple pods to reach the Running phase.
func waitForPodsRunning(t *testing.T, ctx context.Context, clientset *kubernetes.Clientset, podNames []string, timeout time.Duration) bool {
	t.Helper()

	t.Log("Waiting for pods to be Running...")

	success := assert.EventuallyWithTf(t, func(collect *assert.CollectT) {
		for _, podName := range podNames {
			pod, err := clientset.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				collect.Errorf("Error fetching pod %s: %v", podName, err)
				return
			}

			if pod.Status.Phase != corev1.PodRunning {
				collect.Errorf("Pod %s is not running, current phase: %v", podName, pod.Status.Phase)
			}
		}
	}, timeout, 2*time.Second, "pods did not reach Running state")

	if !success {
		for _, podName := range podNames {
			pod, err := clientset.CoreV1().Pods(defaultNamespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil || pod.Status.Phase != corev1.PodRunning {
				logPodEvents(t, ctx, clientset, podName)
			}
		}
	}

	return success
}

// logPodEvents logs the events for a pod (useful for debugging failures).
func logPodEvents(t *testing.T, ctx context.Context, clientset *kubernetes.Clientset, podName string) {
	t.Helper()

	events, err := clientset.CoreV1().Events(defaultNamespace).List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + podName,
	})
	if err != nil {
		t.Logf("Failed to fetch events for pod %s: %v", podName, err)
		return
	}

	if len(events.Items) == 0 {
		t.Logf("No events found for pod %s", podName)
		return
	}

	t.Logf("Events for pod %s:", podName)
	for _, event := range events.Items {
		t.Logf("  [%s] %s: %s", event.Type, event.Reason, event.Message)
	}
}

// assertPodLogsContain checks that the pod logs contain all expected strings.
func assertPodLogsContain(t *testing.T, ctx context.Context, clientset *kubernetes.Clientset, podName string, expectedStrings []string, timeout time.Duration) {
	t.Helper()

	t.Logf("Verifying logs for pod %s...", podName)

	assert.EventuallyWithTf(t, func(collect *assert.CollectT) {
		logs, err := clientset.CoreV1().Pods(defaultNamespace).GetLogs(podName, &corev1.PodLogOptions{}).DoRaw(ctx)
		if err != nil {
			collect.Errorf("Failed to fetch logs for pod %s: %v", podName, err)
			return
		}

		logStr := string(logs)
		for _, expected := range expectedStrings {
			if !strings.Contains(logStr, expected) {
				collect.Errorf("Pod %s logs do not contain %q. Logs: %s", podName, expected, logStr)
			}
		}
	}, timeout, 2*time.Second, "pod %s logs verification failed", podName)
}

// getPodLogs returns the logs of a pod.
func getPodLogs(t *testing.T, ctx context.Context, clientset *kubernetes.Clientset, podName string) string {
	t.Helper()

	logs, err := clientset.CoreV1().Pods(defaultNamespace).GetLogs(podName, &corev1.PodLogOptions{}).DoRaw(ctx)
	require.NoError(t, err, "Failed to fetch logs for pod %s", podName)

	return string(logs)
}
