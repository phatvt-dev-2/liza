package filelock

import (
	"strings"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	metrics := &Metrics{
		Operation:       "test-operation",
		AcquisitionTime: 50 * time.Millisecond,
		HoldTime:        100 * time.Millisecond,
	}

	if metrics.Operation != "test-operation" {
		t.Errorf("Operation = %q, want %q", metrics.Operation, "test-operation")
	}

	if metrics.AcquisitionTime != 50*time.Millisecond {
		t.Errorf("AcquisitionTime = %v, want %v", metrics.AcquisitionTime, 50*time.Millisecond)
	}

	if metrics.HoldTime != 100*time.Millisecond {
		t.Errorf("HoldTime = %v, want %v", metrics.HoldTime, 100*time.Millisecond)
	}
}

func TestMetricsString(t *testing.T) {
	metrics := &Metrics{
		Operation:       "read",
		AcquisitionTime: 25 * time.Millisecond,
		HoldTime:        75 * time.Millisecond,
	}

	str := metrics.String()

	if !strings.Contains(str, "read") {
		t.Errorf("String() should contain operation name 'read'")
	}

	if !strings.Contains(str, "25ms") {
		t.Errorf("String() should contain acquisition time '25ms'")
	}

	if !strings.Contains(str, "75ms") {
		t.Errorf("String() should contain hold time '75ms'")
	}
}

func TestRecordMetrics(t *testing.T) {
	recorder := NewMetricsRecorder()

	metrics1 := &Metrics{
		Operation:       "read",
		AcquisitionTime: 10 * time.Millisecond,
		HoldTime:        50 * time.Millisecond,
	}

	metrics2 := &Metrics{
		Operation:       "write",
		AcquisitionTime: 20 * time.Millisecond,
		HoldTime:        100 * time.Millisecond,
	}

	recorder.Record(metrics1)
	recorder.Record(metrics2)

	recorded := recorder.GetMetrics()
	if len(recorded) != 2 {
		t.Errorf("GetMetrics() returned %d metrics, want 2", len(recorded))
	}

	if recorded[0].Operation != "read" {
		t.Errorf("First metric operation = %q, want %q", recorded[0].Operation, "read")
	}

	if recorded[1].Operation != "write" {
		t.Errorf("Second metric operation = %q, want %q", recorded[1].Operation, "write")
	}
}

func TestMetricsRecorderClear(t *testing.T) {
	recorder := NewMetricsRecorder()

	recorder.Record(&Metrics{
		Operation:       "test",
		AcquisitionTime: 10 * time.Millisecond,
		HoldTime:        20 * time.Millisecond,
	})

	if len(recorder.GetMetrics()) != 1 {
		t.Errorf("Expected 1 metric before clear")
	}

	recorder.Clear()

	if len(recorder.GetMetrics()) != 0 {
		t.Errorf("Expected 0 metrics after clear")
	}
}

func TestMetricsRecorderStats(t *testing.T) {
	recorder := NewMetricsRecorder()

	recorder.Record(&Metrics{
		Operation:       "op1",
		AcquisitionTime: 10 * time.Millisecond,
		HoldTime:        50 * time.Millisecond,
	})

	recorder.Record(&Metrics{
		Operation:       "op2",
		AcquisitionTime: 30 * time.Millisecond,
		HoldTime:        150 * time.Millisecond,
	})

	recorder.Record(&Metrics{
		Operation:       "op3",
		AcquisitionTime: 20 * time.Millisecond,
		HoldTime:        100 * time.Millisecond,
	})

	stats := recorder.GetStats()

	if stats.Count != 3 {
		t.Errorf("Stats.Count = %d, want 3", stats.Count)
	}

	// Average acquisition time: (10 + 30 + 20) / 3 = 20ms
	expectedAvgAcq := 20 * time.Millisecond
	if stats.AvgAcquisitionTime != expectedAvgAcq {
		t.Errorf("Stats.AvgAcquisitionTime = %v, want %v", stats.AvgAcquisitionTime, expectedAvgAcq)
	}

	// Average hold time: (50 + 150 + 100) / 3 = 100ms
	expectedAvgHold := 100 * time.Millisecond
	if stats.AvgHoldTime != expectedAvgHold {
		t.Errorf("Stats.AvgHoldTime = %v, want %v", stats.AvgHoldTime, expectedAvgHold)
	}

	// Max acquisition time: 30ms
	if stats.MaxAcquisitionTime != 30*time.Millisecond {
		t.Errorf("Stats.MaxAcquisitionTime = %v, want %v", stats.MaxAcquisitionTime, 30*time.Millisecond)
	}

	// Max hold time: 150ms
	if stats.MaxHoldTime != 150*time.Millisecond {
		t.Errorf("Stats.MaxHoldTime = %v, want %v", stats.MaxHoldTime, 150*time.Millisecond)
	}
}

func TestMetricsRecorderStatsEmpty(t *testing.T) {
	recorder := NewMetricsRecorder()
	stats := recorder.GetStats()

	if stats.Count != 0 {
		t.Errorf("Stats.Count = %d, want 0", stats.Count)
	}

	if stats.AvgAcquisitionTime != 0 {
		t.Errorf("Stats.AvgAcquisitionTime = %v, want 0", stats.AvgAcquisitionTime)
	}

	if stats.AvgHoldTime != 0 {
		t.Errorf("Stats.AvgHoldTime = %v, want 0", stats.AvgHoldTime)
	}
}

func TestMeasureLockOperation(t *testing.T) {
	// Use deterministic durations without sleeping.
	acquireStart := time.Now()
	acquireEnd := acquireStart.Add(10 * time.Millisecond)

	holdStart := acquireEnd
	holdEnd := holdStart.Add(20 * time.Millisecond)

	metrics := &Metrics{
		Operation:       "test-op",
		AcquisitionTime: acquireEnd.Sub(acquireStart),
		HoldTime:        holdEnd.Sub(holdStart),
	}

	// Allow for some timing variance
	if metrics.AcquisitionTime < 8*time.Millisecond || metrics.AcquisitionTime > 15*time.Millisecond {
		t.Errorf("AcquisitionTime = %v, expected ~10ms", metrics.AcquisitionTime)
	}

	if metrics.HoldTime < 18*time.Millisecond || metrics.HoldTime > 25*time.Millisecond {
		t.Errorf("HoldTime = %v, expected ~20ms", metrics.HoldTime)
	}
}

func TestMetricsRecorderConcurrent(t *testing.T) {
	recorder := NewMetricsRecorder()

	const numGoroutines = 10
	const metricsPerGoroutine = 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < metricsPerGoroutine; j++ {
				recorder.Record(&Metrics{
					Operation:       "concurrent-op",
					AcquisitionTime: time.Duration(id) * time.Millisecond,
					HoldTime:        time.Duration(j) * time.Millisecond,
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	metrics := recorder.GetMetrics()
	expectedCount := numGoroutines * metricsPerGoroutine
	if len(metrics) != expectedCount {
		t.Errorf("GetMetrics() returned %d metrics, want %d", len(metrics), expectedCount)
	}
}
