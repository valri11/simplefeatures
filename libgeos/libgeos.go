package libgeos

// Package libgeos provides a cgo wrapper around the GEOS (Geometry Engine,
// Open Source) library.
//
// Its purpose is to provide functionality that has been implemented in GEOS,
// but is not yet available in the simplefeatures library.
//
// This package can be used in two ways:
//
// 1. Many GEOS non-threadsafe handles can be created, and functionality used
// via those handles. This is useful if many threads need to perform geometry
// operations concurrently (each thread should use its own handle). This method
// of accessing GEOS functionality allows parallelism, but is more difficult to
// use.
//
// 2. The Global functions can be used, which share an unexported global
// handle. Usage is serialised using a mutex, so these functions are safe to
// use concurrently. This method of accessing GEOS functionaliy is easier,
// although doesn't allow parallelism.
//
// The operations in this package ignore Z and M values if they are present.

import (
	"sync"

	"github.com/peterstace/simplefeatures/geom"
)

var (
	globalMutex  sync.Mutex
	globalHandle *Handle
)

func getGlobalHandle() (*Handle, error) {
	if globalHandle != nil {
		return globalHandle, nil
	}

	var err error
	globalHandle, err = NewHandle()
	if err != nil {
		return nil, err
	}
	return globalHandle, nil
}

// Equals returns true if and only if the input geometries are spatially equal.
func Equals(g1, g2 geom.Geometry) (bool, error) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	h, err := getGlobalHandle()
	if err != nil {
		return false, err
	}
	return h.Equals(g1, g2)
}

// TODO:
//
// -- Disjoint
// -- Touches
// -- Contains
// -- Covers
//
// -- Intersects
// -- Within
// -- CoveredBy
//
// -- Crosses
// -- Overlaps
