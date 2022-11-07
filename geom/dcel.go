package geom

import (
	"fmt"
)

type doublyConnectedEdgeList struct {
	faces     []*faceRecord // only populated in the overlay
	halfEdges []*halfEdgeRecord
	vertices  map[XY]*vertexRecord
}

type faceRecord struct {
	cycle *halfEdgeRecord

	// inSet encodes whether this face is part of the input geometry for each
	// operand.
	inSet [2]bool

	extracted bool
}

func (f *faceRecord) String() string {
	if f == nil {
		return "nil"
	}
	return "[" + f.cycle.String() + "]"
}

type halfEdgeRecord struct {
	origin     *vertexRecord
	twin       *halfEdgeRecord
	incident   *faceRecord // only populated in the overlay
	next, prev *halfEdgeRecord
	seq        Sequence

	// srcEdge encodes whether or not this edge is explicitly appears as part
	// of the input geometries.
	srcEdge [2]bool

	// srcFace encodes whether or not this edge explicitly borders onto a face
	// in the input geometries.
	srcFace [2]bool

	// inSet encodes whether or not this edge is (explicitly or implicitly)
	// part of the input geometry for each operand.
	inSet [2]bool

	extracted bool
}

// String shows the origin and destination of the edge (for debugging
// purposes). We can remove this once DCEL active development is completed.
func (e *halfEdgeRecord) String() string {
	if e == nil {
		return "nil"
	}
	return fmt.Sprintf("%v", sequenceToXYs(e.seq))
}

type vertexRecord struct {
	coords    XY
	incidents []*halfEdgeRecord

	// src encodes whether on not this vertex explicitly appears in the input
	// geometries.
	src [2]bool

	// inSet encodes whether or not this vertex is part of each input geometry
	// (although it might not be explicitly encoded there).
	inSet [2]bool

	locations [2]location
	extracted bool
}

func forEachEdge(start *halfEdgeRecord, fn func(*halfEdgeRecord)) {
	e := start
	for {
		fn(e)
		e = e.next
		if e == start {
			break
		}
	}
}

func newDCELFromGeometry(g Geometry, ghosts MultiLineString, operand operand, interactions map[XY]struct{}) *doublyConnectedEdgeList {
	dcel := &doublyConnectedEdgeList{
		vertices: make(map[XY]*vertexRecord),
	}
	dcel.addGeometry(g, operand, interactions)
	dcel.addGhosts(ghosts, operand, interactions)
	return dcel
}

func (d *doublyConnectedEdgeList) addGeometry(g Geometry, operand operand, interactions map[XY]struct{}) {
	switch g.Type() {
	case TypePolygon:
		poly := g.MustAsPolygon()
		d.addMultiPolygon(poly.AsMultiPolygon(), operand, interactions)
	case TypeMultiPolygon:
		mp := g.MustAsMultiPolygon()
		d.addMultiPolygon(mp, operand, interactions)
	case TypeLineString:
		mls := g.MustAsLineString().AsMultiLineString()
		d.addMultiLineString(mls, operand, interactions)
	case TypeMultiLineString:
		mls := g.MustAsMultiLineString()
		d.addMultiLineString(mls, operand, interactions)
	case TypePoint:
		mp := g.MustAsPoint().AsMultiPoint()
		d.addMultiPoint(mp, operand)
	case TypeMultiPoint:
		mp := g.MustAsMultiPoint()
		d.addMultiPoint(mp, operand)
	case TypeGeometryCollection:
		gc := g.MustAsGeometryCollection()
		d.addGeometryCollection(gc, operand, interactions)
	default:
		panic(fmt.Sprintf("unknown geometry type: %v", g.Type()))
	}
}

func (d *doublyConnectedEdgeList) addMultiPolygon(mp MultiPolygon, operand operand, interactions map[XY]struct{}) {
	mp = mp.ForceCCW()

	for polyIdx := 0; polyIdx < mp.NumPolygons(); polyIdx++ {
		poly := mp.PolygonN(polyIdx)

		// Extract rings.
		rings := make([]Sequence, 1+poly.NumInteriorRings())
		rings[0] = poly.ExteriorRing().Coordinates()
		for i := 0; i < poly.NumInteriorRings(); i++ {
			rings[i+1] = poly.InteriorRingN(i).Coordinates()
		}

		// Populate vertices.
		for _, ring := range rings {
			for i := 0; i < ring.Length(); i++ {
				xy := ring.GetXY(i)
				if _, ok := interactions[xy]; !ok {
					continue
				}
				if _, ok := d.vertices[xy]; !ok {
					vr := &vertexRecord{
						coords:    xy,
						incidents: nil, // populated later
						locations: newLocationsOnBoundary(operand),
					}
					vr.src[operand] = true
					d.vertices[xy] = vr
				}
			}
		}

		for _, ring := range rings {
			var newEdges []*halfEdgeRecord
			forEachNonInteractingSegment(ring, interactions, func(segment Sequence) {
				// Build the edges (fwd and rev).
				reverseSegment := reverseSequence(segment)
				vertA := d.vertices[segment.GetXY(0)]
				vertB := d.vertices[reverseSegment.GetXY(0)]
				internalEdge := &halfEdgeRecord{
					origin:   vertA,
					twin:     nil, // populated later
					incident: nil, // only populated in the overlay
					next:     nil, // populated later
					prev:     nil, // populated later
					seq:      segment,
				}
				externalEdge := &halfEdgeRecord{
					origin:   vertB,
					twin:     internalEdge,
					incident: nil, // only populated in the overlay
					next:     nil, // populated later
					prev:     nil, // populated later
					seq:      reverseSegment,
				}
				internalEdge.srcEdge[operand] = true
				internalEdge.srcFace[operand] = true
				externalEdge.srcEdge[operand] = true
				internalEdge.twin = externalEdge
				vertA.incidents = append(vertA.incidents, internalEdge)
				vertB.incidents = append(vertB.incidents, externalEdge)
				newEdges = append(newEdges, internalEdge, externalEdge)
			})

			// Link together next/prev pointers.
			numEdges := len(newEdges)
			for i := 0; i < numEdges/2; i++ {
				newEdges[i*2+0].next = newEdges[(2*i+2)%numEdges]
				newEdges[i*2+1].next = newEdges[(2*i-1+numEdges)%numEdges]
				newEdges[i*2+0].prev = newEdges[(2*i-2+numEdges)%numEdges]
				newEdges[i*2+1].prev = newEdges[(2*i+3)%numEdges]
			}
			d.halfEdges = append(d.halfEdges, newEdges...)
		}
	}
}

func (d *doublyConnectedEdgeList) addMultiLineString(mls MultiLineString, operand operand, interactions map[XY]struct{}) {
	// Add vertices.
	for i := 0; i < mls.NumLineStrings(); i++ {
		ls := mls.LineStringN(i)
		seq := ls.Coordinates()
		n := seq.Length()
		for j := 0; j < n; j++ {
			xy := seq.GetXY(j)
			if _, ok := interactions[xy]; !ok {
				continue
			}

			onBoundary := (j == 0 || j == n-1) && !ls.IsClosed()
			if v, ok := d.vertices[xy]; !ok {
				var locs [2]location
				if onBoundary {
					locs[operand].boundary = true
				} else {
					locs[operand].interior = true
				}
				vr := &vertexRecord{
					coords:    xy,
					locations: locs,
				}
				vr.src[operand] = true
				d.vertices[xy] = vr
			} else {
				if onBoundary {
					if v.locations[operand].boundary {
						// Handle mod-2 rule (the boundary passes through the point
						// an even number of times, then it should be treated as an
						// interior point).
						v.locations[operand].boundary = false
						v.locations[operand].interior = true
					} else {
						v.locations[operand].boundary = true
						v.locations[operand].interior = false
					}
				} else {
					v.locations[operand].interior = true
				}
			}
		}
	}

	edges := make(edgeSet)

	// Add edges.
	for i := 0; i < mls.NumLineStrings(); i++ {
		seq := mls.LineStringN(i).Coordinates()
		forEachNonInteractingSegment(seq, interactions, func(segment Sequence) {
			reverseSegment := reverseSequence(segment)
			startXY := segment.GetXY(0)
			endXY := reverseSegment.GetXY(0)

			if edges.containsStartIntermediateEnd(segment) {
				return
			}
			edges.insertStartIntermediateEnd(segment)
			edges.insertStartIntermediateEnd(reverseSegment)

			vOrigin := d.vertices[startXY]
			vDestin := d.vertices[endXY]

			fwd := &halfEdgeRecord{
				origin:   vOrigin,
				twin:     nil, // set later
				incident: nil, // only populated in overlay
				next:     nil, // set later
				prev:     nil, // set later
				seq:      segment,
			}
			rev := &halfEdgeRecord{
				origin:   vDestin,
				twin:     fwd,
				incident: nil, // only populated in overlay
				next:     fwd,
				prev:     fwd,
				seq:      reverseSegment,
			}
			fwd.srcEdge[operand] = true
			rev.srcEdge[operand] = true
			fwd.twin = rev
			fwd.next = rev
			fwd.prev = rev

			vOrigin.incidents = append(vOrigin.incidents, fwd)
			vDestin.incidents = append(vDestin.incidents, rev)

			d.halfEdges = append(d.halfEdges, fwd, rev)
		})
	}
}

func (d *doublyConnectedEdgeList) addMultiPoint(mp MultiPoint, operand operand) {
	n := mp.NumPoints()
	for i := 0; i < n; i++ {
		xy, ok := mp.PointN(i).XY()
		if !ok {
			continue
		}
		record, ok := d.vertices[xy]
		if !ok {
			record = &vertexRecord{
				coords:    xy,
				incidents: nil,
				locations: [2]location{}, // set below
			}
			d.vertices[xy] = record
		}
		record.src[operand] = true
		record.locations[operand].interior = true
	}
}

func (d *doublyConnectedEdgeList) addGeometryCollection(gc GeometryCollection, operand operand, interactions map[XY]struct{}) {
	n := gc.NumGeometries()
	for i := 0; i < n; i++ {
		d.addGeometry(gc.GeometryN(i), operand, interactions)
	}
}

func (d *doublyConnectedEdgeList) addGhosts(mls MultiLineString, operand operand, interactions map[XY]struct{}) {
	edges := make(edgeSet)
	for _, e := range d.halfEdges {
		edges.insertEdge(e)
	}

	for i := 0; i < mls.NumLineStrings(); i++ {
		seq := mls.LineStringN(i).Coordinates()
		forEachNonInteractingSegment(seq, interactions, func(segment Sequence) {
			reverseSegment := reverseSequence(segment)
			startXY := segment.GetXY(0)
			endXY := reverseSegment.GetXY(0)

			if _, ok := d.vertices[startXY]; !ok {
				d.vertices[startXY] = &vertexRecord{coords: startXY}
			}
			if _, ok := d.vertices[endXY]; !ok {
				d.vertices[endXY] = &vertexRecord{coords: endXY}
			}

			if edges.containsStartIntermediateEnd(segment) {
				// Already exists, so shouldn't add.
				return
			}
			edges.insertStartIntermediateEnd(segment)
			edges.insertStartIntermediateEnd(reverseSegment)

			d.addGhostLine(segment, reverseSegment, operand)
		})
	}
}

func (d *doublyConnectedEdgeList) addGhostLine(segment, reverseSegment Sequence, operand operand) {
	vertA := d.vertices[segment.GetXY(0)]
	vertB := d.vertices[reverseSegment.GetXY(0)]

	e1 := &halfEdgeRecord{
		origin:   vertA,
		twin:     nil, // populated later
		incident: nil, // only populated in the overlay
		next:     nil, // popluated later
		prev:     nil, // populated later
		seq:      segment,
	}
	e2 := &halfEdgeRecord{
		origin:   vertB,
		twin:     e1,
		incident: nil, // only populated in the overlay
		next:     e1,
		prev:     e1,
		seq:      reverseSegment,
	}
	e1.twin = e2
	e1.next = e2
	e1.prev = e2

	vertA.incidents = append(vertA.incidents, e1)
	vertB.incidents = append(vertB.incidents, e2)

	d.halfEdges = append(d.halfEdges, e1, e2)

	d.fixVertex(vertA)
	d.fixVertex(vertB)
}

func forEachNonInteractingSegment(seq Sequence, interactions map[XY]struct{}, fn func(Sequence)) {
	n := seq.Length()
	i := 0
	for i < n-1 {
		// Find the next interaction point after i. This will be the
		// end of the next non-interacting segment.
		start := i
		var end int
		for j := i + 1; j < n; j++ {
			if _, ok := interactions[seq.GetXY(j)]; ok {
				end = j
				break
			}
		}

		// Execute the callback with the segment.
		segment := seq.Slice(start, end+1)
		fn(segment)

		// On the next iteration, start the next edge at the end of
		// this one.
		i = end
	}
}
