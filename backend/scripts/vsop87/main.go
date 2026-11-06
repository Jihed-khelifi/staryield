// Command vsop87 converts the VSOP87B planetary series (Bretagnon & Francou,
// A&A 202, 309, 1988) into the compact, truncated binary files embedded by
// internal/astro/ephem.
//
// Usage (from backend/):
//
//	go run ./scripts/vsop87 -out internal/astro/ephem/data
//	go run ./scripts/vsop87 -src /path/with/VSOP87B.files -out internal/astro/ephem/data
//
// Without -src the eight VSOP87B.{mer,ven,ear,mar,jup,sat,ura,nep} files are
// downloaded from the authors' institute (IMCCE) with the CDS VizieR mirror
// at the Harvard-Smithsonian CfA as a fallback (catalogue VI/81).
//
// Terms whose effective amplitude A·Tmax^k (Tmax = 0.25 Julian millennia,
// i.e. the years 1750–2250) is below 1e-8 (radians for L and B, AU for R)
// are dropped: against JPL Horizons the results stay within 0.05″ of the full
// series while the data shrinks to 30% and evaluation is three times faster.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var bodies = []string{"mer", "ven", "ear", "mar", "jup", "sat", "ura", "nep"}

var mirrors = []string{
	"https://ftp.imcce.fr/pub/ephem/planets/vsop87/",
	"https://vizier.cfa.harvard.edu/ftp/cats/VI/81/",
}

const (
	tMax             = 0.25 // Julian millennia from J2000 covered with full accuracy
	defaultThreshold = 1e-8 // minimum effective amplitude kept
	magic            = "VSOP87B1"
)

type term struct{ A, B, C float64 }

// threshold is the minimum effective amplitude kept (set from -threshold).
var threshold *float64

func main() {
	src := flag.String("src", "", "directory holding VSOP87B.* files (downloaded when empty)")
	out := flag.String("out", "internal/astro/ephem/data", "output directory")
	threshold = flag.Float64("threshold", defaultThreshold, "minimum effective amplitude kept")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	totalKept, totalAll := 0, 0
	for _, b := range bodies {
		raw, err := read(*src, b)
		if err != nil {
			log.Fatalf("%s: %v", b, err)
		}
		series, all, err := parse(raw)
		if err != nil {
			log.Fatalf("%s: %v", b, err)
		}
		kept := 0
		for v := range series {
			for k := range series[v] {
				series[v][k] = truncate(series[v][k], k)
				kept += len(series[v][k])
			}
		}
		totalKept += kept
		totalAll += all
		buf, err := encode(series)
		if err != nil {
			log.Fatalf("%s: %v", b, err)
		}
		path := filepath.Join(*out, "vsop87b_"+b+".bin.gz")
		if err := os.WriteFile(path, buf, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: kept %d of %d terms, %d bytes\n", b, kept, all, len(buf))
	}
	fmt.Printf("total: kept %d of %d terms\n", totalKept, totalAll)
}

func read(src, body string) ([]byte, error) {
	name := "VSOP87B." + body
	if src != "" {
		return os.ReadFile(filepath.Join(src, name))
	}
	client := &http.Client{Timeout: 3 * time.Minute}
	var lastErr error
	for _, m := range mirrors {
		resp, err := client.Get(m + name)
		if err != nil {
			lastErr = err
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && resp.StatusCode == http.StatusOK {
			return data, nil
		}
		lastErr = fmt.Errorf("%s: status %d, %v", m+name, resp.StatusCode, err)
	}
	return nil, lastErr
}

// parse returns series[variable][power] for the L, B and R variables.
func parse(raw []byte) ([3][][]term, int, error) {
	var out [3][][]term
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	v, k := -1, -1
	all := 0
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "VSOP87") {
			// " VSOP87 VERSION B2    EARTH     VARIABLE 1 (LBR)       *T**0    623 TERMS ..."
			i := strings.Index(line, "VARIABLE")
			j := strings.Index(line, "*T**")
			if i < 0 || j < 0 {
				return out, 0, fmt.Errorf("bad header %q", line)
			}
			vv, err1 := strconv.Atoi(strings.TrimSpace(line[i+8 : i+11]))
			kk, err2 := strconv.Atoi(strings.TrimSpace(line[j+4 : j+6]))
			if err1 != nil || err2 != nil || vv < 1 || vv > 3 || kk < 0 || kk > 5 {
				return out, 0, fmt.Errorf("bad header %q", line)
			}
			v, k = vv-1, kk
			for len(out[v]) <= k {
				out[v] = append(out[v], nil)
			}
			continue
		}
		f := strings.Fields(line)
		if len(f) < 3 || v < 0 {
			continue
		}
		var t term
		var err error
		if t.A, err = strconv.ParseFloat(f[len(f)-3], 64); err != nil {
			return out, 0, err
		}
		if t.B, err = strconv.ParseFloat(f[len(f)-2], 64); err != nil {
			return out, 0, err
		}
		if t.C, err = strconv.ParseFloat(f[len(f)-1], 64); err != nil {
			return out, 0, err
		}
		out[v][k] = append(out[v][k], t)
		all++
	}
	return out, all, sc.Err()
}

func truncate(s []term, power int) []term {
	scale := math.Pow(tMax, float64(power))
	kept := s[:0]
	for _, t := range s {
		if math.Abs(t.A)*scale >= *threshold {
			kept = append(kept, t)
		}
	}
	return kept
}

// encode writes: magic, then for each of the 3 variables a series count
// (uint8) followed by, per series, a term count (uint32) and the terms as
// little-endian float64 triples (A, B, C); the whole stream is gzipped.
func encode(series [3][][]term) ([]byte, error) {
	var raw bytes.Buffer
	raw.WriteString(magic)
	for v := range series {
		raw.WriteByte(byte(len(series[v])))
		for _, s := range series[v] {
			if err := binary.Write(&raw, binary.LittleEndian, uint32(len(s))); err != nil {
				return nil, err
			}
			for _, t := range s {
				if err := binary.Write(&raw, binary.LittleEndian, [3]float64{t.A, t.B, t.C}); err != nil {
					return nil, err
				}
			}
		}
	}
	var out bytes.Buffer
	zw, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	zw.ModTime = time.Time{} // reproducible output
	if _, err := zw.Write(raw.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
