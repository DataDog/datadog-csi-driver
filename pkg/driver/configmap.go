// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package driver

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	ConfigMapName = "datadog-csi-driver-config"

	// Keys in the ConfigMap
	KeyVersion    = "version"
	KeySSIEnabled = "ssi_enabled"
)

// DriverConfig holds the configuration to publish
type DriverConfig struct {
	Version    string
	SSIEnabled bool
}

// PublishConfigMap creates or updates a ConfigMap with driver configuration.
// This allows other components (like the cluster-agent) to discover the driver's capabilities.
func PublishConfigMap(ctx context.Context, config DriverConfig) error {
	namespace, err := getNamespace()
	if err != nil {
		return fmt.Errorf("failed to determine namespace: %w", err)
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ssiEnabledStr := "false"
	if config.SSIEnabled {
		ssiEnabledStr = "true"
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "datadog-csi-driver",
				"app.kubernetes.io/managed-by": "datadog-csi-driver",
			},
		},
		Data: map[string]string{
			KeyVersion:    config.Version,
			KeySSIEnabled: ssiEnabledStr,
		},
	}

	_, err = clientset.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		// ConfigMap already exists, update it
		_, err = clientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("failed to create/update configmap: %w", err)
	}

	slog.Info("Published driver configuration to ConfigMap",
		"namespace", namespace,
		"name", ConfigMapName,
		"version", config.Version,
		"ssi_enabled", ssiEnabledStr,
	)

	return nil
}

// getNamespace returns the namespace the pod is running in.
// It reads from the service account token mount, which is always present in-cluster.
func getNamespace() (string, error) {
	// Read from service account (standard in-cluster location)
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", fmt.Errorf("cannot read namespace from service account: %w", err)
	}
	return string(data), nil
}
