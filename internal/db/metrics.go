package db

import (
	"fmt"
	"sync"
	"time"
)

// lockMetrics captures timing information for a lock operation
type lockMetrics struct {
	Operation       string        // Name of the operation that held the lock
	AcquisitionTime time.Duration // Time spent waiting to acquire the lock
	HoldTime        time.Duration // Time the lock was held
}

// String returns a formatted string representation of the metrics
func (m *lockMetrics) String() string {
	return fmt.Sprintf("Lock metrics [%s]: acquisition=%v, hold=%v",
		m.Operation,
		m.AcquisitionTime.Round(time.Millisecond),
		m.HoldTime.Round(time.Millisecond))
}

// lockMetricsRecorder collects lock metrics for analysis
type lockMetricsRecorder struct {
	mu      sync.RWMutex
	metrics []*lockMetrics
}

// newLockMetricsRecorder creates a new metrics recorder
func newLockMetricsRecorder() *lockMetricsRecorder {
	return &lockMetricsRecorder{
		metrics: make([]*lockMetrics, 0),
	}
}

// Record adds a metric to the recorder
func (r *lockMetricsRecorder) Record(m *lockMetrics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = append(r.metrics, m)
}

// GetMetrics returns all recorded metrics
func (r *lockMetricsRecorder) GetMetrics() []*lockMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*lockMetrics, len(r.metrics))
	copy(result, r.metrics)
	return result
}

// Clear removes all recorded metrics
func (r *lockMetricsRecorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = make([]*lockMetrics, 0)
}

// lockMetricsStats contains aggregate statistics
type lockMetricsStats struct {
	Count              int
	AvgAcquisitionTime time.Duration
	AvgHoldTime        time.Duration
	MaxAcquisitionTime time.Duration
	MaxHoldTime        time.Duration
}

// GetStats computes aggregate statistics from recorded metrics
func (r *lockMetricsRecorder) GetStats() lockMetricsStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.metrics) == 0 {
		return lockMetricsStats{}
	}

	var totalAcq, totalHold time.Duration
	var maxAcq, maxHold time.Duration

	for _, m := range r.metrics {
		totalAcq += m.AcquisitionTime
		totalHold += m.HoldTime

		if m.AcquisitionTime > maxAcq {
			maxAcq = m.AcquisitionTime
		}
		if m.HoldTime > maxHold {
			maxHold = m.HoldTime
		}
	}

	count := len(r.metrics)
	return lockMetricsStats{
		Count:              count,
		AvgAcquisitionTime: totalAcq / time.Duration(count),
		AvgHoldTime:        totalHold / time.Duration(count),
		MaxAcquisitionTime: maxAcq,
		MaxHoldTime:        maxHold,
	}
}

// String returns a formatted string representation of the stats
func (s lockMetricsStats) String() string {
	return fmt.Sprintf("Lock statistics: count=%d, avg_acq=%v, avg_hold=%v, max_acq=%v, max_hold=%v",
		s.Count,
		s.AvgAcquisitionTime.Round(time.Millisecond),
		s.AvgHoldTime.Round(time.Millisecond),
		s.MaxAcquisitionTime.Round(time.Millisecond),
		s.MaxHoldTime.Round(time.Millisecond))
}
