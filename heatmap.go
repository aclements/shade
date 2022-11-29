package main

import (
	"image/color"
	"math"
	"sort"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/plotter"
)

func (o *IntensityOverTime) HeatMap() *plot.Plot {
	plt := o.newPlot()
	// The default plot.TimeTicks are terrible, so we compute our own.
	xticks := dayOfYearTicks{}
	plt.X.Tick.Marker = xticks
	plt.X.Label.Text = "Day of year"
	plt.Title.Text = "Sun exposure through year (W/m²)"
	yticks := timeOfDayTicks{6}
	plt.Y.Tick.Marker = yticks
	plt.Y.Label.Text = "Time of day"
	o.heatMap(plt, false)
	return plt
}

func (o *IntensityOverTime) ShadeDuration() *plot.Plot {
	plt := o.newPlot()
	xticks := dayOfYearTicks{}
	plt.X.Tick.Marker = xticks
	plt.X.Label.Text = "Day of year"
	plt.Title.Text = "Sun duration through year"
	yticks := durationTicks{6}
	plt.Y.Tick.Marker = yticks
	plt.Y.Label.Text = "Duration"
	o.heatMap(plt, true)
	return plt
}

func (o *IntensityOverTime) heatMap(plt *plot.Plot, sorted bool) {
	type xy struct {
		day       time.Time
		tod       time.Duration
		sun       SunPos
		intensity float64
		col, row  int
	}

	// Compute the visual locations on the heat map of each sunPos and
	// figure out the bounds of the heat map. We construct columns to
	// start from 0, but for the row range, we narrow down to just the
	// lit times.
	var cMax, rMin, rMax int
	rMax = -1
	startDay, _ := splitTime(o.sunPos[0].T)
	var startTOD time.Duration
	xys := make([]xy, len(o.sunPos))
	for i, sun := range o.sunPos {
		xy := &xys[i]
		xy.day, xy.tod = splitTime(sun.T)
		xy.sun = sun
		xy.intensity = sun.GlobalIntensity(o.elevationFeet)
		xy.col = int(xy.day.Sub(startDay) / (24 * time.Hour))
		xy.row = int(xy.tod / o.increment)
		if xy.col > cMax {
			cMax = xy.col
		}
		if xy.intensity > 0 {
			first := rMax == -1
			if first || xy.row < rMin {
				rMin = xy.row
				startTOD = xy.tod
			}
			if first || xy.row > rMax {
				rMax = xy.row
			}
		}
	}

	if sorted {
		// Rearrange the points so they're sorted by intensity within
		// each day.
		var byDay [][]xy
		last := 0
		for i := range xys {
			if i > 0 && xys[i].col != xys[last].col {
				byDay = append(byDay, xys[last:i])
				last = i
			}
		}
		byDay = append(byDay, xys[last:])
		rMin = 0
		rMax = 0
		startTOD = 0
		cat := func(t xy) int {
			// Full sun first, then foliage, then shade, then darkness.
			switch {
			case t.sun.Altitude < 0:
				return 3
			case t.sun.Foliage:
				return 1
			case t.sun.Light >= 0.05:
				return 0
			}
			return 2
		}
		for _, day := range byDay {
			sort.Slice(day, func(i, j int) bool {
				if c1, c2 := cat(day[i]), cat(day[j]); c1 != c2 {
					return c1 < c2
				}
				return day[i].intensity > day[j].intensity
			})
			// Recompute row
			for i := range day {
				day[i].row = i
				day[i].tod = o.increment * time.Duration(i)
				if i > rMax && day[i].intensity > 0 {
					rMax = i
				}
			}
		}
	}

	// Construct the grid.
	var intensity [][]float64
	var foliage [][]float64
	for i := range xys {
		xy := &xys[i]
		for xy.col >= len(intensity) {
			intensity = append(intensity, make([]float64, rMax-rMin+1))
			foliage = append(foliage, make([]float64, rMax-rMin+1))
		}
		if xy.row < rMin || xy.row > rMax {
			continue
		}
		if xy.sun.Altitude < 0 {
			intensity[xy.col][xy.row-rMin] = -1
			foliage[xy.col][xy.row-rMin] = math.NaN()
		} else if xy.sun.Foliage {
			intensity[xy.col][xy.row-rMin] = math.NaN()
			foliage[xy.col][xy.row-rMin] = xy.intensity
		} else {
			intensity[xy.col][xy.row-rMin] = xy.intensity
			foliage[xy.col][xy.row-rMin] = math.NaN()
		}
	}
	grid := &sunIntensityGrid{intensity, startDay, startTOD, o.increment}
	fGrid := &sunIntensityGrid{foliage, startDay, startTOD, o.increment}

	// Finally, construct the heat map.
	pal := palette.Heat(256, 1)
	hm := plotter.NewHeatMap(grid, pal)
	hm.Underflow = color.Black
	hm.Overflow = color.White
	hm.NaN = color.Transparent
	// Even in vector formats, a rasterized heatmap makes more sense.
	hm.Rasterized = true
	plt.Add(hm)

	fPal := foliagePalette{pal}
	hm = plotter.NewHeatMap(fGrid, fPal)
	hm.NaN = color.Transparent
	hm.Rasterized = true
	plt.Add(hm)

	// Construct a legend.
	thumbs := plotter.PaletteThumbnailers(pal)
	plt.Legend.Add("Full shade", thumbs[0])
	//plt.Legend.Add("Partial shade", thumbs[len(thumbs)/2])
	plt.Legend.Add("Direct sun", thumbs[len(thumbs)-1])
	thumbs = plotter.PaletteThumbnailers(fPal)
	plt.Legend.Add("Foliage shade", thumbs[0])
}

type sunIntensityGrid struct {
	intensity [][]float64
	startDay  time.Time
	startTOD  time.Duration
	increment time.Duration
}

func (si *sunIntensityGrid) Dims() (c, r int) {
	if len(si.intensity) == 0 {
		return 0, 0
	}
	return len(si.intensity), len(si.intensity[0])
}

func (si *sunIntensityGrid) Z(c, r int) float64 {
	return si.intensity[c][r]
}

func (si *sunIntensityGrid) X(c int) float64 {
	t := si.startDay.Add(time.Duration(c) * (24 * time.Hour))
	return float64(t.Unix())
}

func (si *sunIntensityGrid) Y(r int) float64 {
	t := si.startTOD + time.Duration(r)*si.increment
	return float64(t)
}

func (si *sunIntensityGrid) Min() float64 {
	// We use -1 when the sun isn't in the sky to render in the underflow color.
	return 0
}

func (si *sunIntensityGrid) Max() float64 {
	// Solar radiation at sea level on the equator at noon.
	//
	// TODO: It's higher at higher elevations. Maybe I should just let
	// gonum find the max?
	return 1042
}

type foliagePalette struct {
	p palette.Palette
}

func (p foliagePalette) Colors() []color.Color {
	sub := p.p.Colors()
	out := make([]color.Color, len(sub))
	for i, c := range sub {
		r, g, b, a := c.RGBA()
		out[i] = color.RGBA64{uint16(g), uint16(r / 2), uint16(b), uint16(a)}
	}
	return out
}
