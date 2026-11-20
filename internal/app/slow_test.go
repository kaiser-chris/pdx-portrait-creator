package app

import (
	"testing"
	"time"
)

// countFrames counts frames at a rate for a while, from a time, and returns
// the time after.
func countFrames(f *frameRate, from time.Time, perSecond int, lasting time.Duration, judged bool) time.Time {
	step := time.Second / time.Duration(perSecond)

	for at := time.Duration(0); at < lasting; at += step {
		f.frame(from.Add(at), judged)
	}

	return from.Add(lasting)
}

// A portrait drawn at a few frames a second is slow once a window of them is
// counted, stays slow until the rate is well above where it turned slow, and
// is fast again then.
func TestFrameRateWarnsOfASlowPortrait(t *testing.T) {
	var f frameRate

	now := countFrames(&f, time.Unix(0, 0), 60, 6*time.Second, true)
	if f.slow {
		t.Fatalf("60 frames a second judged slow, at %.1f", f.rate)
	}

	now = countFrames(&f, now, 4, 6*time.Second, true)
	if !f.slow || f.rate > slowBelow {
		t.Fatalf("4 frames a second judged fast, at %.1f", f.rate)
	}

	now = countFrames(&f, now, 25, 6*time.Second, true)
	if !f.slow {
		t.Errorf("25 frames a second, between the two rates, took the warning away at %.1f", f.rate)
	}

	countFrames(&f, now, 60, 11*time.Second, true)
	if f.slow {
		t.Errorf("60 frames a second again left the warning at %.1f", f.rate)
	}
}

// Frames of a window with nothing to judge by, such as a minimized one, count
// for nothing: a window minimized for a while is not judged slow for the
// frames it did not draw meanwhile.
func TestFrameRateIgnoresAMinimizedWindow(t *testing.T) {
	var f frameRate

	now := countFrames(&f, time.Unix(0, 0), 60, 2*time.Second, true)
	now = countFrames(&f, now, 1, 30*time.Second, false)
	countFrames(&f, now, 60, 6*time.Second, true)

	if f.slow {
		t.Errorf("a window minimized for half a minute was judged slow, at %.1f", f.rate)
	}
}
