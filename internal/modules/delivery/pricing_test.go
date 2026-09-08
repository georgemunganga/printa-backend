package delivery

import (
	"math"
	"testing"
)

func TestCalculateRuleFee(t *testing.T) {
	tests := []struct {
		name                         string
		base, perKM, above, distance float64
		want                         float64
	}{
		{name: "fixed local tier", base: 30, distance: 2.8, want: 30},
		{name: "fixed middle tier", base: 50, distance: 8.2, want: 50},
		{name: "first kilometre beyond twenty", base: 100, perKM: 5, above: 20, distance: 20.1, want: 105},
		{name: "five kilometres beyond twenty", base: 100, perKM: 5, above: 20, distance: 25, want: 125},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := calculateRuleFee(test.base, test.perKM, test.above, test.distance); got != test.want {
				t.Fatalf("calculateRuleFee() = %.2f, want %.2f", got, test.want)
			}
		})
	}
}

func TestHaversineKM(t *testing.T) {
	distance := haversineKM(-15.4167, 28.2833, -15.3875, 28.3228)
	if math.Abs(distance-5.32) > 0.15 {
		t.Fatalf("haversineKM() = %.2f, want about 5.32", distance)
	}
}
