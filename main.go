package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"text/template"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var (
	povTemplate = template.Must(template.New("pov").Parse(`
#version 3.7;

#include "colors.inc"
#include "sunpos.inc"

global_settings {
	ambient_light White
	assumed_gamma 1.0
}
  
background { color Blue }

camera {
  location <10*12,15*12,-15*12>
  look_at <10*12,0,10*12>
}

#macro testLit(Y,M,D, H,Min, Lstm, TestPos)
  #local Sun = SunPos(Y,M,D, H,Min, Lstm, {{.Lat}},{{.Lon}});
  #local Norm = <0,0,0>;
  #local Intersect = trace(scene, TestPos, Sun - TestPos, Norm);
  // Return true if TestPos is lit bit the sun.
  (vlength(Norm)=0) 
#end
#fopen TESTOUT "{{.OutPath}}" write
#declare TestPos = <{{index .TestPos 0}}, {{index .TestPos 1}}, {{index .TestPos 2}}>;

#declare scene =
`))

	testSceneTemplate = template.Must(template.New("").Parse(`
#declare sunpos = SunPos(2022,11,8, 15,00, 0, 42.4195011,-71.2064993);

object {
	scene
	texture {
	  pigment { color Yellow }
	}
  }

  light_source {
	sunpos
	color White
}

sphere {
	TestPos, 6
}
`))
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
	var testPos = [3]float64{8 * 12, 6, 18 * 12}

	mesh, err := ReadSTL(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	var times []time.Time
	t := time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)
	for t.Year() == 2022 {
		times = append(times, t)
		t = t.Add(time.Minute)
	}

	// TODO: Maybe include source of testLit (and ToPOV?)?
	ck := MakeCacheKey(mesh, lat, lon, testPos, times)
	var isLit []bool
	if !ck.Load(&isLit) {
		isLit = testLit(mesh, lat, lon, testPos, times)
		ck.Save(isLit)
	}

	// Assemble runs
	var runs [][2]time.Time
	for i := 0; i < len(times); {
		if !isLit[i] {
			// Skip to the next lit time.
			for i < len(isLit) && !isLit[i] {
				i++
			}
			continue
		}

		// Gather a run
		j := i
		for j < len(isLit) && isLit[j] && times[i].YearDay() == times[j].YearDay() {
			j++
		}
		runs = append(runs, [2]time.Time{times[i], times[j]})
		i = j
	}

	plt := plot.New()
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
	} else {
		states := make([]state, len(isLit))
		for i, l := range isLit {
			if l {
				states[i] = stateSun
			}
		}
		plotChangesUsingPolys(plt, times, states)
	}
	err = plt.Save(20*vg.Centimeter, 15*vg.Centimeter, "test2.png")
	if err != nil {
		log.Panic(err)
	}
}

func testLit(mesh *Mesh, lat, lon float64, testPos [3]float64, times []time.Time) []bool {
	// As of Pov-Ray 3.7, it only supports input from stdin on DOS (?!)
	src, err := os.CreateTemp("", "shade-*.pov")
	if err != nil {
		log.Fatalf("creating temporary file: %s", err)
	}
	defer os.Remove(src.Name())

	out, err := os.CreateTemp("", "shade-*.out")
	if err != nil {
		log.Fatalf("creating temporary file: %s", err)
	}
	out.Close()
	defer os.Remove(out.Name())

	var tmplArgs struct {
		Lat, Lon float64
		TestPos  [3]float64
		OutPath  string
	}
	tmplArgs.Lat, tmplArgs.Lon = lat, lon
	tmplArgs.TestPos = testPos
	tmplArgs.OutPath = out.Name()
	if err := povTemplate.Execute(src, &tmplArgs); err != nil {
		log.Fatalf("writing POV-Ray input: %s", err)
	}
	if err := mesh.ToPOV(src); err != nil {
		log.Fatalf("writing POV-Ray input: %s", err)
	}
	for _, t := range times {
		// Convert the time to UTC and use a 0 timezone meridian. This
		// is easier than figuring out the meridian. SunPos will
		// complain, but it works fine.
		utc := t.In(time.UTC)
		fmt.Fprintf(src, "#write(TESTOUT, testLit(%d,%d,%d, %d,%d, %d, TestPos))\n", utc.Year(), utc.Month(), utc.Day(), utc.Hour(), utc.Minute(), 0)
	}
	if err := src.Close(); err != nil {
		log.Fatalf("writing POV-Ray input: %s", err)
	}

	// Run povray
	pov := exec.Command("povray",
		"+I"+src.Name(), // Input
		"-D",            // Disable display preview
		"-F",            // Disable file output
		"+H1", "+W1",    // 1x1 pixel output (0x0 isn't supported)
		"-GD", "-GR", "-GS", // Disable most output
	)
	pov.Stdout, pov.Stderr = os.Stdout, os.Stderr
	if err := pov.Run(); err != nil {
		log.Fatalf("running POV-Ray: %s", err)
	}

	// Read the output
	outData, err := os.ReadFile(out.Name())
	if err != nil {
		log.Fatalf("reading shade output file: %s", err)
	}
	isLit := make([]bool, len(outData))
	for i := range outData {
		isLit[i] = outData[i] == '1'
	}
	return isLit
}
