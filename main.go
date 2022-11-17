package main

import (
	"image/color"
	"log"
	"os"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// Notes about SketchUp STL exports:
//
// The SketchUp mesh is in inches.
//
// The red/green/blue SketchUp coordinate system maps to POV like this:
//
//   blue/Y
//     |  green/Z/North
//     | /
//     |/____ red/X

const (
	lat = 42.4195011
	lon = -71.2064993
)

type state uint8

const (
	stateShade state = iota
	stateSun
)

func main() {
	// Y=0 is 90' elevation in the drawings
	var testPos = [3]float64{0, 8 * 12, 0}

	// TODO: Generate a POV rendering showing the test point and the
	// compass directions at some reasonable time and day. Where should
	// the camera be?

	mesh, err := ReadSTL(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	var times []time.Time
	t := time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)
	increment := time.Minute
	for t.Year() == 2022 {
		times = append(times, t)
		t = t.Add(increment)
	}

	// TODO: Maybe include source of ComputeSunPos (and ToPOV?)?
	ck := MakeCacheKey(mesh, lat, lon, testPos, times)
	var sunPos []SunPos
	if !ck.Load(&sunPos) {
		sunPos = ComputeSunPos(mesh, lat, lon, testPos, times)
		ck.Save(sunPos)
	}

	// Assemble runs
	var runs [][2]time.Time
	for i := 0; i < len(sunPos); {
		// Gather a run
		j := i
		for j < len(sunPos) && sunPos[i].Lit == sunPos[j].Lit && sameDay(sunPos[i].T, sunPos[j].T) {
			j++
		}
		if sunPos[i].Lit && j < len(sunPos) {
			runs = append(runs, [2]time.Time{sunPos[i].T, sunPos[j].T})
		}
		i = j
	}

	plt := plot.New()
	// TODO: The default tick marks are *horrible* for time ticks. I
	// guess I'll have to compute those myself. :(
	xticks := plot.TimeTicks{Format: "01-02"}
	plt.X.Tick.Marker = xticks
	yticks := plot.TimeTicks{Format: "3:04PM"}
	plt.Y.Tick.Marker = yticks
	plt.BackgroundColor = color.Black
	for _, elt := range []*color.Color{
		&plt.Title.TextStyle.Color,
		&plt.X.Color,
		&plt.X.Tick.Color,
		&plt.X.Tick.Label.Color,
		&plt.X.Label.TextStyle.Color,
		&plt.Y.Color,
		&plt.Y.Tick.Color,
		&plt.Y.Tick.Label.Color,
		&plt.Y.Label.TextStyle.Color,
	} {
		*elt = color.White
	}
	if false {
		var xys []plotter.XYer
		for _, run := range runs {
			day := time.Date(run[0].Year(), run[0].Month(), run[0].Day(), 0, 0, 0, 0, time.UTC)
			l := float64(day.Unix())
			r := float64(day.Unix() + 60*60*24)
			t := float64(time.Date(2000, 1, 1, run[0].Hour(), run[0].Minute(), run[0].Second(), run[0].Nanosecond(), time.UTC).Unix())
			b := float64(time.Date(2000, 1, 1, run[1].Hour(), run[1].Minute(), run[1].Second(), run[1].Nanosecond(), time.UTC).Unix())
			xys = append(xys, plotter.XYs{
				{X: l, Y: t}, {X: r, Y: t}, {X: r, Y: b}, {X: l, Y: b},
			})
		}
		poly, _ := plotter.NewPolygon(xys...)
		poly.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
		poly.LineStyle.Width = 0
		plt.Add(poly)
	} else if false {
		states := make([]state, len(sunPos))
		for i, sp := range sunPos {
			if sp.Lit {
				states[i] = stateSun
			}
		}
		plotChangesUsingPolys(plt, times, states)
	} else {
		// TODO: Supply elevation
		plotHeatMap(plt, increment, sunPos, 0)
	}
	err = plt.Save(20*vg.Centimeter, 15*vg.Centimeter, "test2.png")
	if err != nil {
		log.Panic(err)
	}
}

var splitTimeDay = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// splitTime splits t into day and time of day. For the day, we put it
// at noon to "center" it on that date. In all cases, we put the result
// in UTC since that's the time zone gonum will render it in and it
// avoids further complications with DST.
func splitTime(t time.Time) (day, tod time.Time) {
	day = time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC)
	tod = time.Date(2000, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	return
}
