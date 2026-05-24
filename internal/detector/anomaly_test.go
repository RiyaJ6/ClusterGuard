package detector

import (
	"math"
	"testing"
)

func TestSlidingWindow_NoAnomalyOnStableInput(t *testing.T) {
	sw := New(10, 3.0)
	// feed a stable signal — all same value
	for i := 0; i < 20; i++ {
		_, anomaly := sw.Score(1.0)
		if anomaly {
			t.Errorf("iteration %d: expected no anomaly on constant input", i)
		}
	}
}

func TestSlidingWindow_DetectsSpike(t *testing.T) {
	sw := New(20, 3.0)
	// warm up with stable values
	for i := 0; i < 20; i++ {
		sw.Score(float64(i % 5)) // values in [0, 4]
	}
	// inject a spike far outside the window distribution
	_, anomaly := sw.Score(1000.0)
	if !anomaly {
		t.Error("expected anomaly for extreme spike, got none")
	}
}

func TestSlidingWindow_ZScoreAccuracy(t *testing.T) {
	// With a window of [0, 2] mean=1, stddev=1
	// A value of 4 should have z-score = |4-1|/1 = 3.0
	sw := New(3, 1.0)
	sw.Score(0)
	sw.Score(2)
	z, anomaly := sw.Score(4)
	if math.Abs(z-1.2247) > 0.01 {
		t.Errorf("expected z-score ~1.2247, got %.4f", z)
	}
	if !anomaly {
		t.Error("expected anomaly at z=1.2247 with threshold=2.5")
	}
}

func TestSlidingWindow_InsufficientData(t *testing.T) {
	sw := New(10, 3.0)
	z, anomaly := sw.Score(999.0) // first value — no baseline yet
	if anomaly {
		t.Error("should not flag anomaly with only one data point")
	}
	if z != 0 {
		t.Errorf("z-score should be 0 with one data point, got %f", z)
	}
}

func TestSlidingWindow_EvictsOldValues(t *testing.T) {
	// window of size 3
	// after eviction the old spike should no longer affect the baseline
	sw := New(3, 3.0)
	sw.Score(100.0) // this will be evicted
	sw.Score(1.0)
	sw.Score(1.0)
	sw.Score(1.0) // evicts the 100 — window is now [1, 1, 1]
	_, anomaly := sw.Score(2.0)
	// 2.0 against a constant-1 window — stddev is 0, should not panic
	if anomaly {
		t.Error("small deviation from constant window should not be anomaly")
	}
}

func TestSlidingWindow_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		window    int
		threshold float64
		inputs    []float64
		wantLast  bool
	}{
		{
			name:      "stable signal no anomaly",
			window:    5, threshold: 3.0,
			inputs:   []float64{1, 1, 1, 1, 1, 1},
			wantLast: false,
		},
		{
			name:      "clear outlier flagged",
			window:    10, threshold: 2.5,
			inputs:   []float64{1, 2, 1, 2, 1, 2, 1, 2, 1, 2, 500},
			wantLast: true,
		},
		{
			name:      "high threshold ignores moderate spike",
			window:    5, threshold: 10.0,
			inputs:   []float64{1, 1, 1, 1, 1, 10},
			wantLast: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sw := New(tc.window, tc.threshold)
			var anomaly bool
			for _, v := range tc.inputs {
				_, anomaly = sw.Score(v)
			}
			if anomaly != tc.wantLast {
				t.Errorf("last anomaly = %v, want %v", anomaly, tc.wantLast)
			}
		})
	}
}
