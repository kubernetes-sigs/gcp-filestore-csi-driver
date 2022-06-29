package metrics

import (
	"testing"
)

const (
	ProcessStartTimeMetric = "process_start_time_seconds"
)

func TestProcessStartTimeMetricExist(t *testing.T) {
	mm := NewMetricsManager()
	metricsFamilies, err := mm.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Error fetching metrics: %v", err)
	}

	// check 'process_start_time_seconds' metric exist
	for _, metricsFamily := range metricsFamilies {
		if metricsFamily.GetName() == ProcessStartTimeMetric {
			return
		}
	}

	t.Fatalf("Metrics does not contain %v. Scraped content: %v", ProcessStartTimeMetric, metricsFamilies)
}
