package main

import (
	"database/sql"
	"testing"

	"github.com/peterstace/simplefeatures/geom"
)

type PostGIS struct {
	db *sql.DB
}

func (p PostGIS) WKTIsValidWithReason(wkt string) (bool, string) {
	var isValid bool
	var reason string
	err := p.db.QueryRow(`
		SELECT
			ST_IsValid(ST_GeomFromText($1)),
			ST_IsValidReason(ST_GeomFromText($1))`,
		wkt,
	).Scan(&isValid, &reason)
	if err != nil {
		// It's not possible to distinguish between problems with the geometry
		// and problems with the database except by string-matching. It's
		// better to just report all errors, even if this means there will be
		// some false errors in the case of connectivity problems (or similar).
		return false, err.Error()
	}
	return isValid, reason
}

func (p PostGIS) WKBIsValidWithReason(t *testing.T, wkb string) (bool, string) {
	var isValid bool
	err := p.db.QueryRow(`SELECT ST_IsValid(ST_GeomFromWKB(decode($1, 'hex')))`, wkb).Scan(&isValid)
	if err != nil {
		return false, err.Error()
	}
	var reason string
	err = p.db.QueryRow(`SELECT ST_IsValidReason(ST_GeomFromWKB(decode($1, 'hex')))`, wkb).Scan(&reason)
	if err != nil {
		return false, err.Error()
	}
	return isValid, reason
}

func (p PostGIS) GeoJSONIsValidWithReason(t *testing.T, geojson string) (bool, string) {
	var isValid bool
	err := p.db.QueryRow(`SELECT ST_IsValid(ST_GeomFromGeoJSON($1))`, geojson).Scan(&isValid)
	if err != nil {
		return false, err.Error()
	}

	var reason string
	err = p.db.QueryRow(`SELECT ST_IsValidReason(ST_GeomFromGeoJSON($1))`, geojson).Scan(&reason)
	if err != nil {
		return false, err.Error()
	}
	return isValid, reason
}

func (p PostGIS) AsText(t *testing.T, g geom.Geometry) string {
	var asText string
	if err := p.db.QueryRow(`SELECT ST_AsText(ST_GeomFromWKB($1))`, g).Scan(&asText); err != nil {
		t.Fatalf("pg error: %v", err)
	}
	return asText
}

func (p PostGIS) AsBinary(t *testing.T, g geom.Geometry) []byte {
	var asBinary []byte
	if err := p.db.QueryRow(`SELECT ST_AsBinary(ST_GeomFromWKB($1))`, g).Scan(&asBinary); err != nil {
		t.Fatalf("pg error: %v", err)
	}
	return asBinary
}

func (p PostGIS) AsGeoJSON(t *testing.T, g geom.Geometry) []byte {
	var geojson []byte
	if err := p.db.QueryRow(`SELECT ST_AsGeoJSON(ST_GeomFromWKB($1))`, g).Scan(&geojson); err != nil {
		t.Fatalf("pg error: %v", err)
	}
	return geojson
}

func (p PostGIS) IsEmpty(t *testing.T, g geom.Geometry) bool {
	var empty bool
	if err := p.db.QueryRow(`
		SELECT ST_IsEmpty(ST_GeomFromWKB($1))`, g,
	).Scan(&empty); err != nil {
		t.Fatalf("pg error: %v", err)
	}
	return empty
}

func (p PostGIS) Dimension(t *testing.T, g geom.Geometry) int {
	var dim int
	if err := p.db.QueryRow(`SELECT ST_Dimension(ST_GeomFromWKB($1))`, g).Scan(&dim); err != nil {
		t.Fatalf("pg error: %v", err)
	}
	return dim
}

func (p PostGIS) Envelope(t *testing.T, g geom.Geometry) geom.Geometry {
	var env geom.AnyGeometry
	if err := p.db.QueryRow(`SELECT ST_AsBinary(ST_Envelope(ST_GeomFromWKB($1)))`, g).Scan(&env); err != nil {
		t.Fatalf("pg error: %v", err)
	}
	return env.Geom
}

func (p PostGIS) IsSimple(t *testing.T, g geom.Geometry) bool {
	var simple bool
	if err := p.db.QueryRow(`SELECT ST_IsSimple(ST_GeomFromWKB($1))`, g).Scan(&simple); err != nil {
		t.Fatalf("pg error: %v", err)
	}
	return simple
}
