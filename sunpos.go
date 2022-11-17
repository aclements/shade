package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"text/template"
	"time"
)

type SunPos struct {
	T   time.Time
	Lit bool // Whether the test point is lit by the sun

	// Altitude is the altitude of the sun in the alt-azimuth coordinate
	// system, in degrees. This ranges from -90 to 90, where 0 is the
	// horizon and 90 is directly overhead.
	Altitude float64
}

// GlobalIntensity computes the total global radiation of the sun (aka
// solar flux, aka insolation) at this position on a plane perpendicular
// to the sun, in W/m².
func (p SunPos) GlobalIntensity(elevationFeet float64) (wattsPerSquareMeter float64) {
	// TODO: Account for transmissivity of foliage if !Lit (maybe I
	// should rename that Direct)

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
	if p.Lit {
		return 1.1 * iDirect
	} else {
		return 0.1 * iDirect
	}
}

func ComputeSunPos(mesh *Mesh, lat, lon float64, testPos [3]float64, times []time.Time) []SunPos {
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
	//defer os.Remove(out.Name())

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
		fmt.Fprintf(src, "testLit(%d,%d,%d, %d,%d, %d, TestPos)\n", utc.Year(), utc.Month(), utc.Day(), utc.Hour(), utc.Minute(), 0)
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
	poses := make([]SunPos, len(times))
	for i, t := range times {
		lit := outData[0] != 0
		al := float64(int32(binary.LittleEndian.Uint32(outData[1:]))) / math.MaxInt32 * 90
		outData = outData[1+4:]
		poses[i] = SunPos{T: t, Lit: lit, Altitude: al}
	}
	return poses
}

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
  #local Lit = (vlength(Norm)=0);
  #write(TESTOUT, uint8 Lit)
  #write(TESTOUT, sint32le Al / 90 * 2147483647)
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
