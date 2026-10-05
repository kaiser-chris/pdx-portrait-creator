package gui

import "testing"

func TestClampScale(t *testing.T) {
	tests := map[float32]float32{
		0.5:  MinScale,
		1:    1,
		1.5:  1.5,
		2.25: 2.25,
		4:    MaxScale,
	}

	for scale, want := range tests {
		if got := clampScale(scale); got != want {
			t.Errorf("clampScale(%v) = %v, want %v", scale, got, want)
		}
	}
}
