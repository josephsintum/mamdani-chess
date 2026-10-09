package server

import (
	"net/netip"
	"testing"
)

func TestOpenGeoWithoutAFile(t *testing.T) {
	g, err := OpenGeo("")
	if g != nil || err != nil {
		t.Fatalf("OpenGeo(\"\") = %v, %v; want nil, nil", g, err)
	}
	if _, err := OpenGeo(t.TempDir() + "/missing.mmdb"); err == nil {
		t.Fatal("a missing file opened")
	}
}

func TestGeoFunc(t *testing.T) {
	var g GeoLookup = geoFunc(func(ip netip.Addr) (string, string) { return "Canada", "Toronto" })
	if c, city := g.Lookup(netip.MustParseAddr("203.0.113.9")); c != "Canada" || city != "Toronto" {
		t.Fatalf("got %q, %q", c, city)
	}
}
