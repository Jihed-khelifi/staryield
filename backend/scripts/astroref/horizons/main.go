// Command horizons fetches reference positions from the JPL Horizons API and
// writes the golden-vector file used by the ephemeris tests.
//
// Usage (from backend/):
//
//	go run ./scripts/astroref/horizons -out internal/astro/ephem/testdata/horizons.json
//
// For every body and instant it records the apparent geocentric ecliptic
// longitude and latitude of date (observer table, QUANTITIES=31, CENTER
// 500@399, UT time scale) together with the query parameters and the date
// of retrieval, so the file documents its own provenance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// bodies maps our names to Horizons body centres (Pluto 999 is the dwarf
// planet itself, not the system barycentre).
var bodies = []struct{ Name, Code string }{
	{"Sun", "10"}, {"Moon", "301"}, {"Mercury", "199"}, {"Venus", "299"}, {"Mars", "499"},
	{"Jupiter", "599"}, {"Saturn", "699"}, {"Uranus", "799"}, {"Neptune", "899"}, {"Pluto", "999"},
}

// instants are spread over 1900–2099 and include leap days, solstices,
// eclipses and dates near sign ingresses (UT).
var instants = []string{
	"1900-03-01T06:15:00Z", "1912-02-29T23:59:00Z", "1923-07-14T12:00:00Z", "1936-10-31T03:30:00Z",
	"1948-12-21T18:00:00Z", "1959-05-05T09:45:00Z", "1966-11-12T21:20:00Z", "1977-03-20T05:00:00Z",
	"1987-11-29T14:30:00Z", "1994-06-21T00:00:00Z", "1999-08-11T11:03:00Z", "2006-04-29T16:40:00Z",
	"2016-09-22T14:21:00Z", "2020-12-21T18:20:00Z", "2031-01-16T07:07:00Z", "2048-08-01T22:10:00Z",
	"2069-04-19T13:13:00Z", "2099-10-15T08:00:00Z",
}

type sample struct {
	Instant string  `json:"instant"`
	Lon     float64 `json:"lon"`
	Lat     float64 `json:"lat"`
}

type bodySet struct {
	Body    string   `json:"body"`
	Horizon string   `json:"horizons_id"`
	Samples []sample `json:"samples"`
}

type file struct {
	Source    string    `json:"source"`
	Query     string    `json:"query"`
	Retrieved string    `json:"retrieved"`
	Bodies    []bodySet `json:"bodies"`
}

func main() {
	out := flag.String("out", "internal/astro/ephem/testdata/horizons.json", "output file")
	flag.Parse()
	jds := make([]string, len(instants))
	for i, s := range instants {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			log.Fatal(err)
		}
		jds[i] = strconv.FormatFloat(float64(t.Unix())/86400+2440587.5, 'f', 9, 64)
	}
	f := file{
		Source:    "JPL Horizons API 1.2 (https://ssd.jpl.nasa.gov/api/horizons.api)",
		Query:     "EPHEM_TYPE=OBSERVER CENTER=500@399 QUANTITIES=31 TIME_TYPE=UT TLIST_TYPE=JD CSV_FORMAT=YES (apparent geocentric ecliptic longitude/latitude of date, degrees)",
		Retrieved: time.Now().UTC().Format(time.DateOnly),
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	for _, b := range bodies {
		q := url.Values{}
		q.Set("format", "text")
		q.Set("COMMAND", "'"+b.Code+"'")
		q.Set("OBJ_DATA", "'NO'")
		q.Set("MAKE_EPHEM", "'YES'")
		q.Set("EPHEM_TYPE", "'OBSERVER'")
		q.Set("CENTER", "'500@399'")
		q.Set("QUANTITIES", "'31'")
		q.Set("TIME_TYPE", "'UT'")
		q.Set("TLIST_TYPE", "'JD'")
		q.Set("CSV_FORMAT", "'YES'")
		q.Set("TLIST", "'"+strings.Join(jds, "' '")+"'")
		resp, err := client.Get("https://ssd.jpl.nasa.gov/api/horizons.api?" + q.Encode())
		if err != nil {
			log.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Fatalf("%s: status %d: %v", b.Name, resp.StatusCode, err)
		}
		samples, err := parse(string(body))
		if err != nil {
			log.Fatalf("%s: %v", b.Name, err)
		}
		if len(samples) != len(instants) {
			log.Fatalf("%s: got %d samples, want %d", b.Name, len(samples), len(instants))
		}
		for i := range samples {
			samples[i].Instant = instants[i]
		}
		f.Bodies = append(f.Bodies, bodySet{Body: b.Name, Horizon: b.Code, Samples: samples})
		fmt.Printf("%s: %d samples\n", b.Name, len(samples))
		time.Sleep(time.Second) // be gentle with the public API
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}

// parse extracts longitude and latitude from the CSV rows between $$SOE and
// $$EOE: "date, , , lon, lat," (solar and lunar presence columns may be empty).
func parse(text string) ([]sample, error) {
	start := strings.Index(text, "$$SOE")
	end := strings.Index(text, "$$EOE")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no ephemeris block in response: %.300s", text)
	}
	var out []sample
	for _, line := range strings.Split(strings.TrimSpace(text[start+5:end]), "\n") {
		cols := strings.Split(line, ",")
		var nums []float64
		for _, c := range cols[1:] {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if v, err := strconv.ParseFloat(c, 64); err == nil {
				nums = append(nums, v)
			}
		}
		if len(nums) < 2 {
			return nil, fmt.Errorf("unexpected row %q", line)
		}
		out = append(out, sample{Lon: nums[0], Lat: nums[1]})
	}
	return out, nil
}
