package main

import "gonum.org/v1/gonum/spatial/r3"

type Mesh struct {
	Verts [][3]float64
	Tris  [][3]int
}

type Ray struct {
	Origin r3.Vec
	Dir    r3.Vec // Must be normalized
}

func (r *Ray) IntersectMesh(m *Mesh) (t float64, ok bool) {
	var tri r3.Triangle
	var minT float64
	haveMin := false
	for _, idxs := range m.Tris {
		for i, idx := range idxs {
			v := m.Verts[idx]
			tri[i] = r3.Vec{X: v[0], Y: v[1], Z: v[2]}
		}
		t, ok := r.IntersectTriangle(&tri)
		if !ok {
			continue
		}
		if !haveMin || t < minT {
			minT, haveMin = t, true
		}
	}
	return minT, haveMin
}

func (r *Ray) IntersectTriangle(tri *r3.Triangle) (t float64, ok bool) {
	// Möller–Trumbore intersection, based on Wikipedia implementation
	// and the Scratchapixel implementation.
	const epsilon = 0.0000001
	edge1 := r3.Sub(tri[1], tri[0])
	edge2 := r3.Sub(tri[2], tri[0])
	h := r3.Cross(r.Dir, edge2)
	det := r3.Dot(edge1, h)
	// If the determinant is negative, this is the "back" of the triangle.
	// If the determinant is close to 0, the ray is parallel to the plane
	// of the triangle.
	if det > -epsilon && det < epsilon {
		return 0, false
	}
	invDet := 1 / det
	s := r3.Sub(r.Origin, tri[0])
	u := invDet * r3.Dot(s, h)
	if u < 0 || u > 1 {
		return 0, false
	}
	q := r3.Cross(s, edge1)
	v := invDet * r3.Dot(r.Dir, q)
	if v < 0 || u+v > 1 {
		return 0, false
	}
	// t is the distance on the ray to the intersection point.
	t = invDet * r3.Dot(edge2, q)
	if t < epsilon {
		// There is a line intersection but not a ray intersection.
		return 0, false
	}
	return t, true
}

func (r *Ray) Along(t float64) r3.Vec {
	return r3.Add(r.Origin, r3.Scale(t, r.Dir))
}
