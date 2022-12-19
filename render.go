package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"text/template"
	"time"
)

func (m *ShadeModel) Render(testPos, cameraOffset [3]float64, t time.Time, outPath string) {
	m.withPOV(testPos, outPath, func(src io.Writer) {
		p := GetSunPos(t, m.lat, m.lon)
		fmt.Fprintf(src, "setSun(%g, %g)\n", p.Altitude, p.Azimuth)
		if err := testSceneTemplate.Execute(src, &cameraOffset); err != nil {
			log.Fatalf("writing POV-Ray input: %s", err)
		}
		for i := range m.layers {
			fmt.Fprintf(src, "object {\n\tmesh%d\n\ttexture { pigment { color White } }\n}\n", i)
		}
	})
}

var testSceneTemplate = template.Must(template.New("").Parse(`
global_settings {
	ambient_light 0
	radiosity {
		pretrace_start 0.08
		pretrace_end   0.01
		count 120
		error_bound 0.25
		recursion_limit 1
	}
	assumed_gamma 1.0
}

sky_sphere{
	pigment{ gradient y
	color_map{
		[0.0 color rgb<1,1,1> ]
		[0.3 color rgb<0.18,0.28,0.75>*0.8]
		[1.0 color rgb<0.15,0.28,0.75>*0.5]}
		scale 1.05
		translate<0,-0.05,0>
	}
}

camera {
	location TestPos + <{{index . 0}}, {{index . 2}}, {{index . 1}}>
	look_at TestPos
}

light_source {
	Sun
	color White
}

sphere {
	TestPos, 6
	texture { pigment { color Green }}
}

// These colors match SketchUp
cylinder {
	TestPos, TestPos + <3*12,0,0>, 3
	texture { pigment { color Red }}
}
cylinder {
	TestPos, TestPos + <0,3*12,0>, 3
	texture { pigment { color Blue }}
}
cylinder {
	TestPos, TestPos + <0,0,3*12>, 3
	texture { pigment { color Green }}
}
text {
    ttf "cyrvetic.ttf" "N" 1, 0
    pigment { Green }
	scale <2*12, 2*12, 1>
	rotate <0, -90, 0>
	translate TestPos + <0, 0, 3*12 + 6>
  }
`))

func (m *ShadeModel) withPOV(testPos [3]float64, output string, cb func(src io.Writer)) {
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

	var tmplArgs struct {
		Lat, Lon float64
		TestPos  [3]float64
	}
	tmplArgs.Lat, tmplArgs.Lon = m.lat, m.lon
	tmplArgs.TestPos = testPos
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
	args = append(args,
		"+O"+output, // Output file
		"+P",        // Pause
		"+A",        // Anti-alias
		//"-D",         // Disable display preview
	)
	pov := exec.Command("povray", args...)
	pov.Stdout, pov.Stderr = os.Stdout, os.Stderr
	if err := pov.Run(); err != nil {
		log.Fatalf("running POV-Ray: %s", err)
	}
}

var povTemplate = template.Must(template.New("pov").Parse(`
#version 3.7;

#include "colors.inc"

#macro setSun(Al, Az)
  #declare Sun = vrotate(<0,0,1000000000>,<-Al,Az,0>);
#end
#declare TestPos = <{{index .TestPos 0}}, {{index .TestPos 2}}, {{index .TestPos 1}}>;
`))
