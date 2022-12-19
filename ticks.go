package main

import (
	"fmt"
	"time"

	"gonum.org/v1/plot"
)

// timeOfDayTicks renders a time.Duration since midnight as a time of day.
type timeOfDayTicks struct {
	targetTicks int // Create around targetTicks number of ticks
}

func (o timeOfDayTicks) Ticks(min, max float64) []plot.Tick {
	minD, maxD := time.Duration(min), time.Duration(max)

	// Find a good duration between ticks
	best, minor := optimizeDurationTicks(minD, maxD, o.targetTicks)

	// Generate ticks and labels.
	var ticks []plot.Tick
	first := int((minD + minor - 1) / minor)
	last := int(maxD / minor)
	minorFactor := int(best / minor)
	var dayBase = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := first; i <= last; i++ {
		t := time.Duration(i) * minor
		label := ""
		if i%minorFactor == 0 {
			label = dayBase.Add(t).Format("3:04PM")
		}
		ticks = append(ticks, plot.Tick{
			Value: float64(t),
			Label: label,
		})
	}
	return ticks
}

type durationTicks struct {
	targetTicks int // Create around targetTicks number of ticks
}

func (o durationTicks) Ticks(min, max float64) []plot.Tick {
	minD, maxD := time.Duration(min), time.Duration(max)
	best, minor := optimizeDurationTicks(minD, maxD, o.targetTicks)

	// Generate ticks and labels.
	//
	// TODO: The math to compute the time.Duration of each tick could be
	// shared better.
	var ticks []plot.Tick
	first := int((minD + minor - 1) / minor)
	last := int(maxD / minor)
	minorFactor := int(best / minor)
	for i := first; i <= last; i++ {
		t := time.Duration(i) * minor
		label := ""
		if i%minorFactor == 0 {
			if best%time.Hour == 0 {
				label = fmt.Sprintf("%dh", int(t.Hours()))
			} else if best%time.Minute == 0 {
				label = fmt.Sprintf("%dh%dm", int(t.Hours()), int(t.Minutes())%60)
			} else {
				label = t.String()
			}
		}
		ticks = append(ticks, plot.Tick{
			Value: float64(t),
			Label: label,
		})
	}
	return ticks
}

var durationScales = []time.Duration{12 * time.Hour, 3 * time.Hour, time.Hour, 30 * time.Minute, 10 * time.Minute, 5 * time.Minute, time.Minute}

func optimizeDurationTicks(minD, maxD time.Duration, targetTicks int) (best, minor time.Duration) {
	// Compute how many ticks would appear in [minD, maxD] for each
	// scale and pick the closest to targetTicks.
	bestNDelta := 0
	for i, scale := range durationScales {
		first := int((minD + scale - 1) / scale)
		last := int(maxD / scale)
		if n := last - first + 1; n > 0 {
			delta := n - targetTicks
			if delta < 0 {
				delta = -delta
			}
			if best == 0 || delta < bestNDelta {
				best, bestNDelta = scale, delta
				if i+1 < len(durationScales) {
					minor = durationScales[i+1]
				} else {
					minor = 0
				}
			}
		}
	}
	if best == 0 {
		best, minor = durationScales[0], durationScales[1]
	}
	return best, minor
}

type dayOfYearTicks struct{}

func (dayOfYearTicks) Ticks(min, max float64) []plot.Tick {
	valToTime := plot.UTCUnixTime
	minT, maxT := valToTime(min), valToTime(max)
	year := minT.Year()
	var ticks []plot.Tick
	lastMajorYear := 0
	for month := time.Month(1); ; month++ {
		t := time.Date(year, month, 1, 12, 0, 0, 0, time.UTC)
		if t.Before(minT) {
			continue
		}
		if t.After(maxT) {
			break
		}
		label := ""
		if (t.Month()-1)%3 == 0 {
			if lastMajorYear == t.Year() {
				label = t.Format("1/02")
			} else {
				lastMajorYear = t.Year()
				label = t.Format("1/02/2006")
			}
		}
		ticks = append(ticks, plot.Tick{
			Value: float64(t.Unix()),
			Label: label,
		})
	}
	return ticks
}

type solsticeTicks struct{}

func (solsticeTicks) Ticks(min, max float64) []plot.Tick {
	valToTime := plot.UTCUnixTime
	minT, maxT := valToTime(min), valToTime(max)
	year := minT.Year()
	var ticks []plot.Tick
	ticks = append(ticks, plot.Tick{
		Value: min,
		Label: minT.Format("1/02/2006"),
	})
	add := func(month time.Month, day int) {
		t := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
		if t.Before(minT) || t.After(maxT) {
			return
		}
		ticks = append(ticks, plot.Tick{
			Value: float64(t.Unix()),
			Label: t.Format("1/02"),
		})
	}
	for ; year <= maxT.Year(); year++ {
		add(3, 20)
		add(6, 21)
		add(9, 22)
		add(12, 22)
	}
	return ticks
}
