package main

import (
	"fmt"
	"io"
	"log"
	"text/template"
	"time"
)

func (m *ShadeModel) Render(testPos, cameraOffset [3]float64, t time.Time, outPath string) {
	m.withPOV(testPos, outPath, func(src io.Writer) {
		utc := t.In(time.UTC)
		fmt.Fprintf(src, "setSun(%d,%d,%d, %d,%d, %d)\n", utc.Year(), utc.Month(), utc.Day(), utc.Hour(), utc.Minute(), 0)
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
