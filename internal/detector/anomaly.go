// Package detector implements a sliding-window z-score anomaly detector.
// It maintains a fixed-size window of recent event scores and flags any
// new value whose z-score exceeds the configured threshold.
//
// Each partition go routine in the consumer gets its own SlidingWindow instance
// so there is no shared mutable state and no locking required.
package detector

import "math"

// SlidingWindow detects anomalies using a z-score over a rolling window.
type SlidingWindow struct {
	window    []float64
	size      int
	threshold float64
	pos       int   // next write position (ring buffer)
	count     int   // how many values have been inserted so far
	sum       float64
	sumSq     float64
}

// New creates a SlidingWindow with the given window size and z-score threshold.
// windowSize must be >= 2.
func New(windowSize int, threshold float64) *SlidingWindow {
	if windowSize < 2 {
		windowSize = 2
	}
	return &SlidingWindow{
		window:    make([]float64, windowSize),
		size:      windowSize,
		threshold: threshold,
	}
}

// Score adds value to the window and returns (zScore, isAnomaly).
// Until the window is full the first time, zScore is always 0 and
// isAnomaly is always false — we need at least 2 samples for variance.
func (sw *SlidingWindow) Score(value float64) (zScore float64, isAnomaly bool) {
	if sw.count >= sw.size {
		// evict the oldest value before inserting
		oldest := sw.window[sw.pos]
		sw.sum -= oldest
		sw.sumSq -= oldest * oldest
	}

	sw.window[sw.pos] = value
	sw.sum += value
	sw.sumSq += value * value
	sw.pos = (sw.pos + 1) % sw.size

	if sw.count < sw.size {
		sw.count++
	}

	if sw.count < 2 {
		return 0, false
	}

	n := float64(sw.count)
	mean := sw.sum / n
	variance := (sw.sumSq / n) - (mean * mean)

	if variance <= 0 {
		return 0, false
	}

	stddev := math.Sqrt(variance)
	zScore = math.Abs(value-mean) / stddev
	return zScore, zScore > sw.threshold
}

// Reset clears the window. Useful between test cases.
func (sw *SlidingWindow) Reset() {
	sw.pos = 0
	sw.count = 0
	sw.sum = 0
	sw.sumSq = 0
	for i := range sw.window {
		sw.window[i] = 0
	}
}
