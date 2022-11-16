package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
)

type Mesh struct {
	Header string

	Verts [][3]float32
	Tris  [][3]int
}

func ReadSTL(r io.Reader) (*Mesh, error) {
	m := new(Mesh)

	var header struct {
		H    [80]byte
		NTri uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}
	m.Header = strings.TrimRight(string(header.H[:]), " ")

	vertMap := make(map[[3]float32]int)

	var vert [3]float32
	var tri [3]int
	triBuf := make([]byte, 4*3*4+2)
	for i := 0; i < int(header.NTri); i++ {
		// Read a triangle
		if _, err := io.ReadFull(r, triBuf); err != nil {
			return nil, err
		}
		// Read the vertexes.
		for v := range tri {
			// Read the coordinates of this vertex.
			for c := range vert {
				const start = 3 * 4 // Skip normal
				vert[c] = math.Float32frombits(binary.LittleEndian.Uint32(triBuf[start+12*v+4*c:]))
			}
			// Add the vertex to the vertex set.
			vertIndex, ok := vertMap[vert]
			if !ok {
				vertIndex = len(m.Verts)
				m.Verts = append(m.Verts, vert)
				vertMap[vert] = vertIndex
			}
			tri[v] = vertIndex
		}
		// Add the triangle.
		m.Tris = append(m.Tris, tri)
	}

	return m, nil
}

func (m *Mesh) ToPOV(w io.Writer) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "mesh2 {\n")
	fmt.Fprintf(&buf, "  vertex_vectors {\n")
	fmt.Fprintf(&buf, "    %d,\n", len(m.Verts))
	for _, vert := range m.Verts {
		// Reorder the vertexes to align the STL coordinate system with POV-Ray
		fmt.Fprintf(&buf, "    <%v, %v, %v>,\n", vert[0], vert[2], vert[1])
	}
	fmt.Fprintf(&buf, "  }\n")
	fmt.Fprintf(&buf, "  face_indices {\n")
	fmt.Fprintf(&buf, "    %d,\n", len(m.Tris))
	for _, tri := range m.Tris {
		fmt.Fprintf(&buf, "    <%d, %d, %d>,\n", tri[0], tri[1], tri[2])
	}
	fmt.Fprintf(&buf, "  }\n")
	fmt.Fprintf(&buf, "}\n")

	_, err := w.Write(buf.Bytes())
	return err
}
