package main

import (
	"image/color"
	"os"
	"time"

	"gonum.org/v1/plot"
)

// A ShadeModel computes the sun exposure on a test point in a 3D model.
//
// The coordinate system is as follows:
//
//	Z/up
//	|  Y/north
//	| /
//	|/____ X/east
type ShadeModel struct {
	lat, lon float64

	elevationFeet float64

	layers []*shadeLayer
}

// NewShadeModel returns a shade model where the origin is at the given
// latitude, longitude, and elevation. Latitude and longitude are in
// degrees, where north and east are positive, respectively. Elevation
// is in feet.
func NewShadeModel(latitude, longitude float64, elevationFeet float64) *ShadeModel {
	return &ShadeModel{
		lat:           latitude,
		lon:           longitude,
		elevationFeet: elevationFeet,
	}
}

type shadeLayer struct {
	mesh *Mesh

	// transmissivity returns the transmissivity of this layer on the
	// given date in a range of 0 to 1. For a fully opaque layer, this
	// returns 0. For foliage, this varies over the year.
	transmissivity func(date time.Time) float64
}

func (m *ShadeModel) AddBuildings(stlPath string) error {
	return m.addLayer(stlPath, func(time.Time) float64 { return 0 })
}

func (m *ShadeModel) AddFoliage(stlPath string) error {
	trans := func(date time.Time) float64 {
		// Based on Transmissivity of solar radiation through crowns of
		// single urban trees—application for outdoor thermal comfort
		// modelling. Konarska, et al.
		//
		// Foliated and defoliated trees have ~5% and ~50%
		// transmissivity, respectively. Use the meteorological seasons
		// to interpolate between these.
		//
		// TODO: This assumes northern hemisphere, and mid-latitudes at
		// that.
		day := date.YearDay()
		const (
			// Assume a normal year. This is all approximate anyway.
			Feb28 = 59
			May31 = 151
			Aug31 = 243
			Nov30 = 334
		)
		switch {
		default: // Winter
			fallthrough
		case day <= Feb28: // Winter
			return 0.5
		case day <= May31: // Spring
			return 0.5 + float64(day-Feb28)/(May31-Feb28)*(0.05-0.5)
		case day <= Aug31: // Summer
			return 0.05
		case day <= Nov30: // Fall
			return 0.05 + float64(day-Aug31)/(Nov30-Aug31)*(0.5-0.05)
		}
	}
	return m.addLayer(stlPath, trans)
}

func (m *ShadeModel) addLayer(stlPath string, trans func(time.Time) float64) error {
	f, err := os.Open(stlPath)
	if err != nil {
		return err
	}
	defer f.Close()
	mesh, err := ReadSTL(f)
	if err != nil {
		return err
	}
	m.layers = append(m.layers, &shadeLayer{mesh, trans})
	return nil
}

type IntensityOverTime struct {
	sunPos []SunPos

	elevationFeet float64
	increment     time.Duration
}

func (m *ShadeModel) IntensityOverYear(year int, testPos [3]float64) *IntensityOverTime {
	// TODO: Generate a POV rendering showing the test point and the
	// compass directions at some reasonable time and day. Where should
	// the camera be? I could put it at testPos and use a panoramic
	// camera. Or I could put it above looking down.

	// TODO: If I compute my own sun positions, I can skip the times
	// below the horizon entirely.
	var times []time.Time
	t := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	increment := time.Minute
	for t.Year() == year {
		times = append(times, t)
		t = t.Add(increment)
	}

	// TODO: Maybe include source of ComputeSunPos (and ToPOV?) in CacheKey?
	var meshes []*Mesh
	for _, l := range m.layers {
		meshes = append(meshes, l.mesh)
	}
	ck := MakeCacheKey(meshes, m.lat, m.lon, testPos, times)
	var sunPos []SunPos
	if !ck.Load(&sunPos) {
		sunPos = m.computeSunPos(testPos, times)
		ck.Save(sunPos)
	}

	return &IntensityOverTime{sunPos, m.elevationFeet, increment}
}

func (o *IntensityOverTime) newPlot() *plot.Plot {
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
	return plt
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
