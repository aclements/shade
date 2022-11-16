package main

import (
	"image/color"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
)

func plotChangesUsingPolys(plt *plot.Plot, times []time.Time, states []state) {
	for _, poly := range makePolys(makeVisualChanges(findChanges(times, states))) {
		poly, _ := plotter.NewPolygon(poly.xys...)
		poly.Color = color.RGBA{R: 255, G: 255, B: 0, A: 255}
		poly.LineStyle.Width = 0
		plt.Add(poly)
	}
}

type change struct {
	t             time.Time
	before, after state
}

func findChanges(times []time.Time, states []state) (changes []change) {
	// Find the times at which states transition.
	for i, t := range times {
		// Check for non-zero states that span days.
		if states[i] != 0 && (i == 0 || !sameDay(times[i-1], times[i])) {
			// Insert a no-op change
			changes = append(changes, change{t, states[i], states[i]})
		}
		if i > 0 && states[i] != states[i-1] {
			changes = append(changes, change{t, states[i-1], states[i]})
		}
	}
	return
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// vChange is a [change] converted into "visual space". We do this
// conversion to simplify logic around DST shifts.
type vChange struct {
	change
	xy plotter.XY
	xi int // Day index
}

func makeVisualChanges(changes []change) (vChanges []vChange) {
	var baseDate = time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	for _, c := range changes {
		t := c.t
		// Split into day and time of day. For the day, we put it at noon to
		// "center" it on that date. In all cases, we put the result in UTC
		// since that's the time zone gonum will render it in.
		day := time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC)
		xi := int(day.Sub(baseDate) / (24 * time.Hour))
		x := day.Unix()
		y := time.Date(2000, 1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC).Unix()
		xy := plotter.XY{X: float64(x), Y: float64(y)}
		vChanges = append(vChanges, vChange{c, xy, xi})
	}
	return
}

func overlaps(a1, a2, b1, b2 vChange) bool {
	if a1.after != a2.before {
		panic("bad change span")
	}
	if b1.after != b2.before {
		panic("bad change span")
	}
	if a1.after != b1.after {
		return false
	}
	return b1.xy.Y < a2.xy.Y && a1.xy.Y < b2.xy.Y
}

type poly struct {
	xys []plotter.XYer
	s   state
}

func makePolys(cs []vChange) []*poly {
	// Create one poly for each state. A poly can consist of several paths.
	polyMap := make(map[state]*poly)
	var polys []*poly
	addEdge := func(xys plotter.XYs, s state) {
		p := polyMap[s]
		if p == nil {
			p = &poly{s: s}
			polyMap[s] = p
			polys = append(polys, p)
		}
		p.xys = append(p.xys, xys)
	}

	// traced tracks which edge points we've traced the right side of
	// while moving in the +X direction.
	traced := make([]bool, len(cs))

	// trace1 traces one edge, starting at cs[startI], starting in the
	// +X direction. If you picture yourself walking the edge, it always
	// follows the "right" side of the edge (for example, if it comes to
	// a three-way intersection). The result will be a clockwise edge
	// for the outside of a polygon and a counter-clockwise edge for a
	// hole.
	//
	// For all of this terminology, we assume we're in quadrant I of a
	// cartesian system (so time increases going up and going right).
	trace1 := func(startI int) plotter.XYs {
		var xys []plotter.XY
		dir := 1
		//fmt.Println("START")
		for i := startI; len(xys) == 0 || i != startI; {
			//fmt.Println(i, cs[i].t, dir, cs[i].before, cs[i].after, cs[i].xi)
			// Add this point
			xys = append(xys, cs[i].xy)
			if dir == 1 {
				// We only record when we're moving right. If you
				// picture two concentric circles, this means we'll do
				// three traces: the two outer edges in CW order and the
				// inner edge (a second time) in CCW order.
				traced[i] = true
			}
			// Find the next point.
			if dir == 1 {
				// Find the latest overlapping segment on xi+1.
				best := -1
				for j := i + 1; j < len(cs) && cs[j].xi <= cs[i].xi+1; j++ {
					if cs[j].xi == cs[i].xi+1 && overlaps(cs[i-1], cs[i], cs[j-1], cs[j]) {
						best = j
					}
				}
				if best != -1 {
					// See if we can bend ever further to the left, into
					// a direction-reversing concavity. E.g., are we
					// coming from X:
					//
					//    \
					//     \
					//    X
					//    /
					if i < len(cs) && cs[i+1].xi == cs[i].xi && cs[best-1].xy.Y < cs[i+1].xy.Y && cs[i+1].xy.Y < cs[best].xy.Y {
						dir = -1
						i = i + 1
					} else {
						i = best
					}
				} else {
					// Follow this edge down and reverse direction
					dir = -1
					i = i - 1
				}
			} else {
				// Find the earliest overlapping segment on xi-1.
				best := -1
				for j := i - 1; j >= 0 && cs[j].xi >= cs[i].xi-1; j-- {
					if cs[j].xi == cs[i].xi-1 && overlaps(cs[i], cs[i+1], cs[j], cs[j+1]) {
						best = j
					}
				}
				if best != -1 {
					if i > 0 && cs[i-1].xi == cs[i].xi && cs[best].xy.Y < cs[i-1].xy.Y && cs[i-1].xy.Y < cs[best+1].xy.Y {
						dir = 1
						i = i - 1
					} else {
						i = best
					}
				} else {
					// Follow this edge up and reverse direction
					dir = 1
					i = i + 1
				}
			}
		}
		return xys
	}

	// Start tracing at each untraced point.
	for i := range cs {
		if traced[i] {
			continue
		}
		// Don't trace state 0. This avoids creating a CCW trace around
		// the outside, which is important, and also avoids tracing any
		// interior state 0 polygons, which are simply unnecessary.
		if cs[i].before == 0 {
			continue
		}

		xys := trace1(i)
		addEdge(xys, cs[i].before)
	}

	return polys
}
