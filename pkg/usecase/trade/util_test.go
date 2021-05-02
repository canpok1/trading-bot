package trade_test

import (
	"testing"
	"trading-bot/pkg/usecase/trade"
)

var EPSILON float64 = 0.00000001

func floatEquals(a, b float64) bool {
	return (a-b) < EPSILON && (b-a) < EPSILON
}

func TestLinFit(t *testing.T) {
	type args struct {
		x []float64
		y []float64
	}
	tests := map[string]struct {
		args  args
		wantA float64
		wantB float64
	}{
		"x and y are empty": {
			args:  args{x: []float64{}, y: []float64{}},
			wantA: 0.0,
			wantB: 0.0,
		},
		"point len is 1": {
			args: args{
				x: []float64{1},
				y: []float64{1},
			},
			wantA: 0.0,
			wantB: 0.0,
		},
		"point len is 2": {
			args: args{
				x: []float64{50, 60, 70, 80, 90},
				y: []float64{40, 70, 90, 60, 100},
			},
			wantA: 1.1,
			wantB: -5.0,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotA, gotB := trade.LinFit(tt.args.x, tt.args.y)
			if !floatEquals(gotA, tt.wantA) {
				t.Errorf("LinFit() gotA = %v, want %v", gotA, tt.wantA)
			}
			if !floatEquals(gotB, tt.wantB) {
				t.Errorf("LinFit() gotB = %v, want %v", gotB, tt.wantB)
			}
		})
	}
}
