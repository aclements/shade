package main

import (
	"log"
	"time"

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

type state uint8

const (
	stateShade state = iota
	stateSun
)

func main() {
	// In this model, Z=0 is the 90' reference on the architectural
	// drawings. That's close to 200' actual elevation.
	const lat = 42.4195011
	const lon = -71.2064993
	const elev = 200
	var testPos = [3]float64{0, 0, 8 * 12}
	var cameraOffset = [3]float64{40 * 12, -30 * 12, 10 * 12}

	m := NewShadeModel(lat, lon, elev)
	m.AddBuildings("house.stl")

	m.Render(testPos, cameraOffset, time.Date(2022, 6, 1, 12, 0, 0, 0, time.Local), "render.png")

	m.AddFoliage("house-trees.stl")

	plt := m.IntensityOverYear(2022, testPos).HeatMap()
	err := plt.Save(20*vg.Centimeter, 15*vg.Centimeter, "sun.png")
	if err != nil {
		log.Panic(err)
	}
}
