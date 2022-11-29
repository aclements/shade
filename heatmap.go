package main

import (
	"image/color"
	"math"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/plotter"
)

func (o *IntensityOverTime) HeatMap() *plot.Plot {
	plt := o.newPlot()
	plt.Title.Text = "Sun exposure through year (W/m²)"

	type xy struct {
		day, tod  time.Time
		intensity float64
		foliage   bool
		col, row  int
	}

	// TODO: Draw an outline around when the shade comes from foliage.

	// Compute the visual locations on the heat map of each sunPos and
	// figure out the bounds of the heat map. We construct columns to
	// start from 0, but for the row range, we narrow down to just the
	// lit times.
	var cMax, rMin, rMax int
	startDay, _ := splitTime(o.sunPos[0].T)
	var startTOD time.Time
	xys := make([]xy, len(o.sunPos))
	for i, sun := range o.sunPos {
		xy := &xys[i]
		xy.day, xy.tod = splitTime(sun.T)
		xy.intensity = sun.GlobalIntensity(o.elevationFeet)
		xy.foliage = sun.Foliage
		xy.col = int(xy.day.Sub(startDay) / (24 * time.Hour))
		xy.row = int(xy.tod.Sub(splitTimeDay) / o.increment)
		if xy.col > cMax {
			cMax = xy.col
		}
		if xy.intensity > 0 {
			first := startTOD.IsZero()
			if first || xy.row < rMin {
				rMin = xy.row
				startTOD = xy.tod
			}
			if first || xy.row > rMax {
				rMax = xy.row
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
		if xy.foliage {
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

	return plt
}

type sunIntensityGrid struct {
	intensity          [][]float64
	startDay, startTOD time.Time
	increment          time.Duration
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
	t := si.startTOD.Add(time.Duration(r) * si.increment)
	return float64(t.Unix())
}

func (si *sunIntensityGrid) Min() float64 {
	// Return 1 rather than 0 so that the "0" value when the sun isn't
	// in the sky renders in the underflow color.
	return 1
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
