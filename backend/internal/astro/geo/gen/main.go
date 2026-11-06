// Command gen builds the embedded gazetteer from the GeoNames dump.
//
// Usage (from backend/internal/astro/geo, or via go generate):
//
//	go run ./gen -out data -legacy-out legacy_gen.go
//	go run ./gen -src /dir/with/geonames/files -out data -legacy-out legacy_gen.go
//
// Inputs (downloaded from https://download.geonames.org/export/dump/ when -src
// is empty): cities5000.zip (every populated place with at least 5,000
// inhabitants, plus administrative seats), admin1CodesASCII.txt and
// countryInfo.txt. GeoNames data is licensed under Creative Commons
// Attribution 4.0; the attribution is shipped in data/NOTICE and returned by
// the API.
//
// Outputs:
//   - data/places.tsv.gz: one place per line: geonameid, name, ASCII name
//     (empty when identical), country code, region (admin1) name, latitude,
//     longitude, population, IANA time zone and, for places of 15,000 people
//     or more, up to six English, French or Spanish alternate names joined by
//     "|" (from alternateNamesV2.zip, given with -altnames);
//   - data/countries.tsv: ISO code and English country name;
//   - data/VERSION: the dump's date;
//   - legacy_gen.go: the map from the old 173-city slugs ("paris-fr") to
//     GeoNames IDs, so stored profiles keep resolving.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Jihed-khelifi/staryield/backend/internal/astro/geo/fold"
)

const base = "https://download.geonames.org/export/dump/"

type place struct {
	id         int
	name       string
	ascii      string
	alts       []string
	lat, lon   float64
	cc         string
	admin1Code string
	population int
	tz         string
	feature    string
}

func main() {
	src := flag.String("src", "", "directory with cities5000.txt (or .zip), admin1CodesASCII.txt and countryInfo.txt; downloaded when empty")
	out := flag.String("out", "data", "output directory")
	legacyOut := flag.String("legacy-out", "legacy_gen.go", "generated legacy mapping file")
	legacyIn := flag.String("legacy", "gen/legacy_places.json", "legacy gazetteer (JSON)")
	version := flag.String("version", "", "dataset version label (default: today's date)")
	altNames := flag.String("altnames", "", "path to alternateNamesV2.zip (or .txt) for English, French and Spanish names; skipped when empty")
	flag.Parse()

	dir := *src
	if dir == "" {
		tmp, err := os.MkdirTemp("", "geonames")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(tmp)
		for _, f := range []string{"cities5000.zip", "admin1CodesASCII.txt", "countryInfo.txt"} {
			if err := download(base+f, filepath.Join(tmp, f)); err != nil {
				log.Fatal(err)
			}
		}
		dir = tmp
	}
	cities, err := readCities(dir)
	if err != nil {
		log.Fatal(err)
	}
	admin1, err := readAdmin1(filepath.Join(dir, "admin1CodesASCII.txt"))
	if err != nil {
		log.Fatal(err)
	}
	countries, err := readCountries(filepath.Join(dir, "countryInfo.txt"))
	if err != nil {
		log.Fatal(err)
	}
	places, err := parseCities(cities)
	if err != nil {
		log.Fatal(err)
	}
	if *altNames != "" {
		if err := addAlternates(*altNames, places); err != nil {
			log.Fatal(err)
		}
	}
	sort.Slice(places, func(i, j int) bool { return places[i].id < places[j].id })
	if *version == "" {
		*version = time.Now().UTC().Format(time.DateOnly)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := writePlaces(filepath.Join(*out, "places.tsv.gz"), places, admin1, countries); err != nil {
		log.Fatal(err)
	}
	if err := writeCountries(filepath.Join(*out, "countries.tsv"), countries); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "VERSION"), []byte("geonames-"+*version+"\n"), 0o644); err != nil {
		log.Fatal(err)
	}
	if err := writeLegacy(*legacyIn, *legacyOut, places); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d places, %d countries, version geonames-%s\n", len(places), len(countries), *version)
}

func download(url, path string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status %d", url, resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func readCities(dir string) ([]byte, error) {
	if data, err := os.ReadFile(filepath.Join(dir, "cities5000.txt")); err == nil {
		return data, nil
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "cities5000.zip"))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "cities5000.txt" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("cities5000.txt not found in zip")
}

func readAdmin1(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, "\t")
		if len(f) >= 2 {
			out[f[0]] = f[1]
		}
	}
	return out, nil
}

func readCountries(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) >= 5 && len(f[0]) == 2 {
			out[f[0]] = f[4]
		}
	}
	return out, nil
}

func parseCities(data []byte) ([]place, error) {
	var out []place
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		if len(f) < 19 {
			continue
		}
		id, err := strconv.Atoi(f[0])
		if err != nil {
			return nil, err
		}
		lat, err1 := strconv.ParseFloat(f[4], 64)
		lon, err2 := strconv.ParseFloat(f[5], 64)
		if err1 != nil || err2 != nil || f[17] == "" {
			continue
		}
		pop, _ := strconv.Atoi(f[14])
		p := place{id: id, name: clean(f[1]), ascii: clean(f[2]), lat: lat, lon: lon, cc: f[8], admin1Code: f[10], population: pop, tz: f[17], feature: f[7]}
		out = append(out, p)
	}
	return out, sc.Err()
}

// clean removes characters that would break the TSV encoding.
func clean(s string) string {
	return strings.NewReplacer("\t", " ", "|", " ", "\n", " ", "\r", " ").Replace(strings.TrimSpace(s))
}

// altLanguages are the languages whose names users of the site type.
var altLanguages = map[string]int{"en": 0, "fr": 1, "es": 2}

const (
	altMinPopulation = 15000 // smaller places rarely have exonyms
	altPerPlace      = 6
)

// addAlternates reads alternateNamesV2 (zip or txt) and attaches up to six
// English, French and Spanish names per place (preferred names first, no
// historic or colloquial names) that fold differently from the main names,
// such as "Londres", "Moscou", "Múnich" or "Pékin".
func addAlternates(path string, places []place) error {
	var r io.Reader
	if strings.HasSuffix(path, ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.Name == "alternateNamesV2.txt" {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				defer rc.Close()
				r = rc
			}
		}
		if r == nil {
			return fmt.Errorf("alternateNamesV2.txt not found in %s", path)
		}
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	index := make(map[int]int, len(places))
	for i, p := range places {
		if p.population >= altMinPopulation {
			index[p.id] = i
		}
	}
	type cand struct {
		name      string
		preferred bool
		lang      int
	}
	cands := map[int][]cand{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		if len(f) < 8 {
			continue
		}
		lang, ok := altLanguages[f[2]]
		if !ok || f[6] == "1" || f[7] == "1" { // colloquial or historic
			continue
		}
		id, err := strconv.Atoi(f[1])
		if err != nil {
			continue
		}
		if _, ok := index[id]; !ok {
			continue
		}
		name := clean(f[3])
		if len(name) > 60 || !fold.IsLatin(name) || len(fold.String(name)) < 2 {
			continue
		}
		cands[id] = append(cands[id], cand{name: name, preferred: f[4] == "1", lang: lang})
	}
	if err := sc.Err(); err != nil {
		return err
	}
	for id, list := range cands {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].preferred != list[j].preferred {
				return list[i].preferred
			}
			return list[i].lang < list[j].lang
		})
		p := &places[index[id]]
		seen := map[string]bool{fold.String(p.name): true, fold.String(p.ascii): true}
		for _, c := range list {
			k := fold.String(c.name)
			if seen[k] {
				continue
			}
			seen[k] = true
			p.alts = append(p.alts, c.name)
			if len(p.alts) == altPerPlace {
				break
			}
		}
	}
	return nil
}

func writePlaces(path string, places []place, admin1, countries map[string]string) error {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return err
	}
	zw.ModTime = time.Time{}
	w := bufio.NewWriter(zw)
	for _, p := range places {
		ascii := p.ascii
		if ascii == p.name {
			ascii = ""
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%.4f\t%.4f\t%d\t%s\t%s\n",
			p.id, p.name, ascii, p.cc, clean(admin1[p.cc+"."+p.admin1Code]),
			p.lat, p.lon, p.population, p.tz, strings.Join(p.alts, "|"))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func writeCountries(path string, countries map[string]string) error {
	codes := make([]string, 0, len(countries))
	for c := range countries {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	var b strings.Builder
	for _, c := range codes {
		fmt.Fprintf(&b, "%s\t%s\n", c, clean(countries[c]))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type legacyPlace struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	CountryCode string   `json:"country_code"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
	Aliases     []string `json:"aliases"`
}

// distanceKm is the great-circle distance between two points.
func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dlat, dlon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

// writeLegacy maps every legacy slug to the most populous GeoNames place of
// the same country whose name (or alias) matches within 60 km, or else to
// the nearest place of that country within 30 km.
func writeLegacy(in, out string, places []place) error {
	raw, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	var legacy []legacyPlace
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	byCountry := map[string][]place{}
	for _, p := range places {
		byCountry[p.cc] = append(byCountry[p.cc], p)
	}
	var b strings.Builder
	b.WriteString("// Code generated by gen; DO NOT EDIT.\n\npackage geo\n\n")
	b.WriteString("// legacyIDs maps the slugs of the original 173-city gazetteer to GeoNames\n// IDs, so birth places stored before the switch keep resolving.\nvar legacyIDs = map[string]int{\n")
	for _, l := range legacy {
		names := map[string]bool{fold.String(l.Name): true}
		for _, a := range l.Aliases {
			names[fold.String(a)] = true
		}
		best, bestPop, bestDist := -1, -1, math.MaxFloat64
		for _, p := range byCountry[l.CountryCode] {
			d := distanceKm(l.Latitude, l.Longitude, p.lat, p.lon)
			match := names[fold.String(p.name)] || names[fold.String(p.ascii)]
			for _, a := range p.alts {
				match = match || names[fold.String(a)]
			}
			if match && d <= 60 && p.population > bestPop {
				best, bestPop, bestDist = p.id, p.population, d
			}
		}
		if best < 0 {
			for _, p := range byCountry[l.CountryCode] {
				if d := distanceKm(l.Latitude, l.Longitude, p.lat, p.lon); d <= 30 && d < bestDist {
					best, bestDist = p.id, d
				}
			}
		}
		if best < 0 {
			return fmt.Errorf("no GeoNames match for legacy place %s", l.ID)
		}
		fmt.Fprintf(&b, "\t%q: %d,\n", l.ID, best)
	}
	b.WriteString("}\n")
	return os.WriteFile(out, []byte(b.String()), 0o644)
}
