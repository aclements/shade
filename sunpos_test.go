package main

import "testing"

// between returns whether x is in [a, b].
func assertBetween(t *testing.T, msg string, x, a, b float64) {
	if a <= x && x <= b {
		return
	}
	t.Errorf("got %s = %v, want in range [%v, %v]", msg, x, a, b)
}

func TestGlobalIntensity(t *testing.T) {
	// These tests are based on the tables at
	// https://www.ftexploring.com/solar-energy/air-mass-and-insolation2.htm

	mkPos := func(alt float64) SunLight {
		return SunLight{Light: 1, SunPos: SunPos{Altitude: alt}}
	}
	p := mkPos(90)
	assertBetween(t, "GlobalIntensity at 90°", p.GlobalIntensity(0), 1041, 1042)

	p = mkPos(1)
	assertBetween(t, "GlobalIntensity at 1°", p.GlobalIntensity(0), 56, 57)

	p = mkPos(0)
	assertBetween(t, "GlobalIntensity at 0°", p.GlobalIntensity(0), 22.4, 22.5)
}
