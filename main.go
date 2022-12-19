package main

import (
	"log"
	"os"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// Notes about SketchUp STL exports:
//
// The SketchUp mesh is in inches.
//
// The red/green/blue SketchUp coordinate system maps to STL like this:
//
//   blue/Z
//     |  green/Y/North
//     | /
//     |/____ red/X

func main() {
	// In this model, Z=0 is the 90' reference on the architectural
	// drawings. That's close to 200' actual elevation.
	const lat = 42.4195011
	const lon = -71.2064993
	const elev = 200
	// Tree in the middle of the lower patio
	//var testPos = [3]float64{0, 0, (5 + 5) * 12}
	// Garage end of green roof
	//var testPos = [3]float64{-15 * 12, -10 * 12, (10 + 5) * 12}
	// House end of green roof
	var testPos = [3]float64{-10 * 12, 8 * 12, (10 + 5) * 12}

	m := NewShadeModel(lat, lon, elev)
	m.AddBuildings("house.stl")

	//var cameraOffset = [3]float64{40 * 12, -30 * 12, 10 * 12}
	//m.Render(testPos, cameraOffset, time.Date(2022, 6, 1, 12, 0, 0, 0, time.Local), "render.png")
	//return

	m.AddFoliage("house-trees.stl")

	intensity := m.IntensityOverYear(2022, testPos)

	plt := intensity.HeatMap()
	writePng(plt, "sun.png")
	plt = intensity.ShadeDuration()
	writePng(plt, "duration.png")
}

func writePng(plt *plot.Plot, path string) {
	c := vgimg.PngCanvas{Canvas: vgimg.NewWith(vgimg.UseWH(20*vg.Centimeter, 15*vg.Centimeter), vgimg.UseDPI(150))}
	plt.Draw(draw.New(c))
	f, err := os.Create(path)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()
	c.WriteTo(f)
}
