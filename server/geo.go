package server

import (
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

// GeoLookup names the country and city an address is in, "" when unknown.
// The server's Geo is nil without a database: then every visit is
// "Unknown", and nothing else changes.
type GeoLookup interface {
	Lookup(ip netip.Addr) (country, city string)
}

// geoFunc lets a plain function serve as a GeoLookup (tests).
type geoFunc func(netip.Addr) (string, string)

func (f geoFunc) Lookup(ip netip.Addr) (string, string) { return f(ip) }

type mmdb struct{ r *maxminddb.Reader }

// OpenGeo opens an MMDB file such as DB-IP's City Lite (dbip-city-lite.mmdb);
// an empty path means no lookups. The file is memory-mapped, so a 130 MB
// database costs no RAM until it is read.
func OpenGeo(path string) (GeoLookup, error) {
	if path == "" {
		return nil, nil
	}
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	return mmdb{r}, nil
}

// Lookup reads the English names; DB-IP and MaxMind both store them under
// country.names.en and city.names.en.
func (m mmdb) Lookup(ip netip.Addr) (country, city string) {
	res := m.r.Lookup(ip)
	if !res.Found() {
		return "", ""
	}
	res.DecodePath(&country, "country", "names", "en")
	res.DecodePath(&city, "city", "names", "en")
	return country, city
}
