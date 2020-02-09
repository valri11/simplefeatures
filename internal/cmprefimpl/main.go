package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"sort"

	"github.com/peterstace/simplefeatures/geom"
	"github.com/peterstace/simplefeatures/internal/libgeos"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get working dir: %v", err)
	}
	candidates, err := extractStringsFromSource(dir)
	if err != nil {
		log.Fatalf("could not extract strings from src: %v", err)
	}

	geoms, err := convertToGeometries(candidates)
	if err != nil {
		log.Fatalf("could not convert to geometries: %v", err)
	}

	geoms = deduplicateGeometries(geoms)

	h, err := libgeos.NewHandle()
	if err != nil {
		panic(err)
	}
	defer h.Close()

	for _, g := range geoms {
		var buf bytes.Buffer
		lg := log.New(&buf, "", log.Lshortfile)
		lg.Printf("========================== START ===========================")
		lg.Printf("WKT: %v", g.AsText())
		err := unaryChecks(h, g, lg)
		lg.Printf("=========================== END ============================")
		if err != nil {
			fmt.Printf("Check failed: %v\n", err)
			io.Copy(os.Stdout, &buf)
			fmt.Println()
		}
	}
}

func deduplicateGeometries(geoms []geom.Geometry) []geom.Geometry {
	m := map[string]geom.Geometry{}
	for _, g := range geoms {
		m[g.AsText()] = g
	}
	geoms = geoms[:0]
	for _, g := range m {
		geoms = append(geoms, g)
	}
	sort.Slice(geoms, func(i, j int) bool {
		return geoms[i].AsText() < geoms[j].AsText()
	})
	return geoms
}
