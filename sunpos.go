package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/sixdouglas/suncalc"
	"gonum.org/v1/gonum/spatial/r3"
)

type SunPos struct {
	T time.Time

	// Altitude is the altitude of the sun in the alt-azimuth coordinate
	// system, in degrees. This ranges from -90 to 90, where 0 is the
	// horizon and 90 is directly overhead.
	Altitude float64

	// Azimuth is the azimuth of the sun in the alt-azimuth coordinate
	// system, in degrees. This ranges from 0 to 360, where 0 is north
	// and 90 is east.
	Azimuth float64
}

// GetSunPos returns the sun position in horizonal alt-azimuth
// coordinates at the given time and location. Latitude and longitude
// are in degrees, where north and east are positive, respectively.
// Elevation is in feet.
func GetSunPos(t time.Time, latitude, longitude float64) SunPos {
	p := suncalc.GetPosition(t, latitude, longitude)
	// suncalc returns angles in radians (even though it takes latitude
	// and longitude in degrees). Also, it uses a non-standard
	// convention for azimuth where -90 is east, 0 is south, 90 is west,
	// and 180 is north.
	const rad2deg = 180 / math.Pi
	return SunPos{t, p.Altitude * rad2deg, p.Azimuth*rad2deg + 180}
}

func (p SunPos) Ray(origin [3]float64) Ray {
	const deg2rad = math.Pi / 180
	al := p.Altitude * deg2rad
	az := p.Azimuth * deg2rad
	return Ray{
		Origin: r3.Vec{X: origin[0], Y: origin[1], Z: origin[2]},
		Dir: r3.Unit(r3.Vec{
			X: math.Sin(az) * math.Cos(al),
			Y: math.Cos(az) * math.Cos(al),
			Z: math.Sin(al),
		}),
	}
}

type SunLight struct {
	SunPos

	Light   float64 // Multiplier of direct illumination, between 0 and 1
	Foliage bool    // This is blocked solely by foliage
}

// GlobalIntensity computes the total global radiation of the sun (aka
// solar flux, aka insolation) at this position on a plane perpendicular
// to the sun, in W/m².
func (p SunLight) GlobalIntensity(elevationFeet float64) (wattsPerSquareMeter float64) {
	// This is based on https://www.pveducation.org/pvcdrom/properties-of-sunlight/air-mass
	if p.Altitude < 0 {
		return 0
	}

	// Compute air mass. This is a unitless number that is between 1 if
	// the sun is directly overhead (minimal air mass) and ~38 if the
	// sun is at the horizon. You'd think we would account for elevation
	// here, but we actually do that in the illumination model. The core
	// of this formula is simply the 1/cos(Θ); the rest of the terms
	// account for the curvature of the Earth.
	//
	// From Kasten, F. and Young, A. T., “Revised optical air mass
	// tables and approximation formula”, Applied Optics, vol. 28, pp.
	// 4735–4738, 1989.
	zenithAngle := 90 - p.Altitude // 0 is overhead
	airMass := 1 / (math.Cos(zenithAngle*(math.Pi/180)) + (0.50572 * math.Pow((96.07995-zenithAngle), -1.6364)))

	// Compute direct component of sunlight, accounting for elevation.
	// From Meinel, A. B. and Meinel, M. P., Applied Solar Energy.
	// Addison Wesley Publishing Co., 1976.
	h := elevationFeet * 0.0003048 // To kilometers
	a := 0.14
	iDirect := 1353 * ((1-a*h)*math.Pow(0.7, math.Pow(airMass, 0.678)) + a*h)

	// Diffuse radiation is ~10% of direct radiation.
	return (0.1 + p.Light) * iDirect
}

func (m *ShadeModel) computeSunLight(testPos [3]float64, times []time.Time) []SunLight {
	// TODO: This could be much more efficient. Do traces in parallel
	// and since I only care about hit tests for this, not exact
	// intersection point, add a fast path that caches the last triangle
	// intersection and retests just that triangle on the next ray.
	light := make([]SunLight, len(times))
	for i, t := range times {
		out := &light[i]
		sunPos := GetSunPos(t, m.lat, m.lon)
		out.SunPos = sunPos
		if sunPos.Altitude < 0 {
			out.Light = 0
			out.Foliage = false
			continue
		}

		sunRay := sunPos.Ray(testPos)
		light := 1.0
		building, foliage := false, false
		for _, l := range m.layers {
			tRay, hit := sunRay.IntersectMesh(l.mesh)
			_ = tRay
			if hit && light != 0 {
				light *= l.transmissivity(t)
			}
			if hit {
				if l.foliage {
					foliage = true
				} else {
					building = true
				}
			}
		}
		out.Light = light
		out.Foliage = foliage && !building
	}
	return light
}

func (m *ShadeModel) withPOV(testPos [3]float64, output string, cb func(src io.Writer)) []byte {
	// The POV-Ray coordinate system looks like:
	//
	//	Y
	//	|  Z/north
	//	| /
	//	|/____ X/east
	//
	// So we have to swap Y and Z between the STL and POV systems

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
	tmplArgs.Lat, tmplArgs.Lon = m.lat, m.lon
	tmplArgs.TestPos = testPos
	tmplArgs.OutPath = out.Name()
	if err := povTemplate.Execute(src, &tmplArgs); err != nil {
		log.Fatalf("writing POV-Ray input: %s", err)
	}
	for i, l := range m.layers {
		fmt.Fprintf(src, "#declare mesh%d = ", i)
		if err := l.mesh.ToPOV(src); err != nil {
			log.Fatalf("writing POV-Ray input: %s", err)
		}
	}
	cb(src)
	if err := src.Close(); err != nil {
		log.Fatalf("writing POV-Ray input: %s", err)
	}

	// Run povray
	args := []string{
		"+I" + src.Name(),   // Input
		"-GD", "-GR", "-GS", // Disable most output
	}
	if output != "" {
		args = append(args,
			"+O"+output, // Output file
			"+P",        // Pause
			"+A",        // Anti-alias
		)
	} else {
		args = append(args, []string{
			"-F",         // Disable file output
			"-D",         // Disable display preview
			"+H1", "+W1", // 1x1 pixel output (0x0 isn't supported)
		}...)
	}
	pov := exec.Command("povray", args...)
	pov.Stdout, pov.Stderr = os.Stdout, os.Stderr
	if err := pov.Run(); err != nil {
		log.Fatalf("running POV-Ray: %s", err)
	}

	// Read the output
	outData, err := os.ReadFile(out.Name())
	if err != nil {
		log.Fatalf("reading shade output file: %s", err)
	}
	return outData
}

var povTemplate = template.Must(template.New("pov").Parse(`
#version 3.7;

#include "colors.inc"

#macro setSun(Al, Az)
  #declare Sun = vrotate(<0,0,1000000000>,<-Al,Az,0>);
#end
#macro testHit(Mesh)
  #local Norm = <0,0,0>;
  #local Intersect = trace(Mesh, TestPos, Sun - TestPos, Norm);
  #local Hit = (vlength(Norm)!=0);
  #write(TESTOUT, uint8 Hit)
#end
#fopen TESTOUT "{{.OutPath}}" write
#declare TestPos = <{{index .TestPos 0}}, {{index .TestPos 2}}, {{index .TestPos 1}}>;
`))
