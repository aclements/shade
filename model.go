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
	// The default plot.TimeTicks are terrible, so we compute our own.
	xticks := dayOfYearTicks{}
	//xticks := solsticeTicks{}
	plt.X.Tick.Marker = xticks
	plt.X.Label.Text = "Day of year"
	yticks := timeOfDayTicks{6}
	plt.Y.Tick.Marker = yticks
	plt.Y.Label.Text = "Time of day"
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

type timeOfDayTicks struct {
	targetTicks int // Create around targetTicks number of ticks
}

var todScales = []time.Duration{12 * time.Hour, 3 * time.Hour, time.Hour, 30 * time.Minute, 10 * time.Minute, 5 * time.Minute, time.Minute}

func (o timeOfDayTicks) Ticks(min, max float64) []plot.Tick {
	valToTime := plot.UTCUnixTime
	minT, maxT := valToTime(min), valToTime(max)
	baseT := time.Date(minT.Year(), minT.Month(), minT.Day(), 0, 0, 0, 0, time.UTC)
	// Compute duration since midnight.
	minD, maxD := minT.Sub(baseT), maxT.Sub(baseT)
	// Compute how many ticks would appear in [minD, maxD] for each
	// scale and pick the closest to targetTicks.
	best, bestNDelta := time.Duration(0), 0
	minor := time.Duration(0)
	for i, scale := range todScales {
		first := int((minD + scale - 1) / scale)
		last := int(maxD / scale)
		if n := last - first + 1; n > 0 {
			delta := n - o.targetTicks
			if delta < 0 {
				delta = -delta
			}
			if best == 0 || delta < bestNDelta {
				best, bestNDelta = scale, delta
				if i+1 < len(todScales) {
					minor = todScales[i+1]
				} else {
					minor = 0
				}
			}
		}
	}
	if best == 0 {
		best, minor = todScales[0], todScales[1]
	}
	// Generate ticks.
	var ticks []plot.Tick
	first := int((minD + minor - 1) / minor)
	last := int(maxD / minor)
	minorFactor := int(best / minor)
	for i := first; i <= last; i++ {
		t := baseT.Add(time.Duration(i) * minor)
		label := ""
		if i%minorFactor == 0 {
			label = t.Format("3:04PM")
		}
		ticks = append(ticks, plot.Tick{
			Value: float64(t.Unix()),
			Label: label,
		})
	}
	return ticks
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
