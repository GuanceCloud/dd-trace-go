// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016 Datadog, Inc.

package profiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"time"
)

type point struct {
	metric string
	value  float64
}

// MarshalJSON serialize points as array tuples
func (p point) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{
		p.metric,
		p.value,
	})
}

type metrics struct {
	collectedAt time.Time
	snapshot    metricsSnapshot
	compute     func(*metricsSnapshot, *metricsSnapshot, time.Duration, time.Time) []point
}

type metricsSnapshot struct {
	runtime.MemStats
	NumGoroutine int
}

func newMetrics() *metrics {
	return &metrics{
		compute: computeMetrics,
	}
}

func (m *metrics) reset(now time.Time) {
	m.collectedAt = now
	runtime.ReadMemStats(&m.snapshot.MemStats)
	m.snapshot.NumGoroutine = runtime.NumGoroutine()
}

func (m *metrics) report(now time.Time, buf *bytes.Buffer) error {
	period := now.Sub(m.collectedAt)

	if period <= 0 {
		// A positive interval is required to compute rates. Stopping the profiler
		// can cause collections to occur within the system clock's resolution.
		return fmt.Errorf(
			"unexpected duration %v between metrics collections, first at %v, second at %v",
			period, m.collectedAt, now,
		)
	}

	previousStats := m.snapshot
	m.reset(now)

	points := m.compute(&previousStats, &m.snapshot, period, now)
	data, err := json.Marshal(removeInvalid(points))

	if err != nil {
		// NB removeInvalid ensures we don't hit this case by dropping inf/NaN
		return err
	}

	if _, err := buf.Write(data); err != nil {
		return err
	}

	return nil
}

func computeMetrics(prev *metricsSnapshot, curr *metricsSnapshot, period time.Duration, now time.Time) []point {
	periodSeconds := float64(period) / float64(time.Second)
	return []point{
		{metric: "go_alloc_bytes_per_sec", value: rate(curr.TotalAlloc, prev.TotalAlloc, periodSeconds)},
		{metric: "go_allocs_per_sec", value: rate(curr.Mallocs, prev.Mallocs, periodSeconds)},
		{metric: "go_frees_per_sec", value: rate(curr.Frees, prev.Frees, periodSeconds)},
		{metric: "go_heap_growth_bytes_per_sec", value: rate(curr.HeapAlloc, prev.HeapAlloc, periodSeconds)},
		{metric: "go_gcs_per_sec", value: rate(uint64(curr.NumGC), uint64(prev.NumGC), periodSeconds)},
		{metric: "go_gc_pause_time", value: rate(curr.PauseTotalNs, prev.PauseTotalNs, float64(period))}, // % of time spent paused
		{metric: "go_max_gc_pause_time", value: float64(maxPauseNs(&curr.MemStats, now.Add(-period)))},
		{metric: "go_num_goroutine", value: float64(curr.NumGoroutine)},
	}
}

func rate(curr, prev uint64, period float64) float64 {
	return float64(int64(curr)-int64(prev)) / period
}

// maxPauseNs returns maximum pause time within the recent period, assumes stats populated at period end
func maxPauseNs(stats *runtime.MemStats, periodStart time.Time) (max uint64) {
	// NB
	// stats.PauseEnd is a circular buffer of recent GC pause end times as nanoseconds since the epoch.
	// stats.PauseNs is a circular buffer of recent GC pause times in nanoseconds.
	// The most recent pause is indexed by (stats.NumGC+255)%256

	for i := 0; i < 256; i++ {
		offset := (int(stats.NumGC) + 255 - i) % 256
		// Stop searching if we find a PauseEnd outside the period
		if time.Unix(0, int64(stats.PauseEnd[offset])).Before(periodStart) {
			break
		}
		if stats.PauseNs[offset] > max {
			max = stats.PauseNs[offset]
		}
	}
	return max
}

// removeInvalid removes NaN and +/-Inf values as they can't be json-serialized
// This is an extra safety check to ensure we don't emit bad data in case of
// a metric computation coding error
func removeInvalid(points []point) (result []point) {
	for _, p := range points {
		if math.IsNaN(p.value) || math.IsInf(p.value, 0) {
			continue
		}
		result = append(result, p)
	}
	return result
}
