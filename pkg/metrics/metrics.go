// Datadog datadog-csi driver
// Copyright 2025-present Datadog, Inc.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator   = "_"
	subsystem   = "datadog_csi_driver"
	MetricsPort = 5000
)

// Status represents the status of an operation
type Status string

const (
	// StatusSuccess represents the success of an operation
	StatusSuccess Status = "success"
	// StatusFailed represents the failure of an operation
	StatusFailed = "failed"
	// StatusUnsupported represents an operation not supported by any publisher
	StatusUnsupported = "unsupported"
)

func newCounterVec(name, help string, labels ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: subsystem + separator + name,
		Help: help,
	}, labels)
}

var nodeVolumeStageAttempts = newCounterVec(
	"node_stage_volume_attempts",
	"Counts the number of stage volume requests received by the csi node server",
	"type",
	"path",
	"status",
)

var nodeVolumeUnstageAttempts = newCounterVec(
	"node_unstage_volume_attempts",
	"Counts the number of unstage volume requests received by the csi node server",
	"status",
)

var nodeVolumeMountAttempts = newCounterVec(
	"node_publish_volume_attempts",
	"Counts the number of publish volume requests received by the csi node server",
	"type",
	"path",
	"status",
)

var nodeVolumeUnmountAttempts = newCounterVec(
	"node_unpublish_volume_attempts",
	"Counts the number of unpublish volume requests received by the csi node server",
	"status",
)

func init() {
	prometheus.MustRegister(nodeVolumeStageAttempts)
	prometheus.MustRegister(nodeVolumeUnstageAttempts)
	prometheus.MustRegister(nodeVolumeMountAttempts)
	prometheus.MustRegister(nodeVolumeUnmountAttempts)
}

// RecordVolumeStageAttempt records a volume stage attempt
func RecordVolumeStageAttempt(volumeType, path string, status Status) {
	nodeVolumeStageAttempts.WithLabelValues(volumeType, path, string(status)).Inc()
}

// RecordVolumeUnstageAttempt records a volume unstage attempt
func RecordVolumeUnstageAttempt(status Status) {
	nodeVolumeUnstageAttempts.WithLabelValues(string(status)).Inc()
}

// RecordVolumeMountAttempt records a volume mount attempt
func RecordVolumeMountAttempt(volumeType, path string, status Status) {
	nodeVolumeMountAttempts.WithLabelValues(volumeType, path, string(status)).Inc()
}

// RecordVolumeUnMountAttempt records a volume unmount attempt
func RecordVolumeUnMountAttempt(status Status) {
	nodeVolumeUnmountAttempts.WithLabelValues(string(status)).Inc()
}
