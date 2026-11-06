"""Generate independent house-cusp reference charts with the Swiss Ephemeris.

This is an offline development tool: it is NOT part of the server and the
Swiss Ephemeris (AGPL) is never linked into Staryield's code. It only writes
reference numbers that internal/astro tests compare against.

    python3 -m venv /tmp/swe && /tmp/swe/bin/pip install pyswisseph==2.10.3.2
    /tmp/swe/bin/python scripts/astroref/houses_swisseph.py > internal/astro/testdata/houses_swisseph.json

Charts cover both hemispheres, the equator, 60°N and the polar circle where
Placidus is undefined (the polar charts use Porphyry, the fallback).
"""

import datetime
import json

import swisseph as swe

CHARTS = [
    # name, UTC instant, latitude, longitude (east positive), house systems
    ("Paris", "1987-11-29T13:30:00Z", 48.8566, 2.3522, "PWE"),
    ("New York", "1970-01-01T05:00:00Z", 40.7128, -74.0060, "PWE"),
    ("Tokyo", "2011-03-11T05:46:00Z", 35.6762, 139.6503, "PWE"),
    ("Sydney", "1995-06-15T08:00:00Z", -33.8688, 151.2093, "PWE"),
    ("Buenos Aires", "1980-10-05T18:20:00Z", -34.6037, -58.3816, "PWE"),
    ("Cape Town", "2003-02-14T21:10:00Z", -33.9249, 18.4241, "PWE"),
    ("Ushuaia", "1999-12-31T23:59:00Z", -54.8019, -68.3030, "PWE"),
    ("Quito", "1960-07-20T12:00:00Z", -0.1807, -78.4678, "PWE"),
    ("Oslo", "1962-03-21T06:00:00Z", 59.9139, 10.7522, "PWE"),
    ("Helsinki", "2001-12-01T23:45:00Z", 60.1699, 24.9384, "PWE"),
    ("Reykjavik", "1975-08-08T17:17:00Z", 64.1466, -21.9426, "PWE"),
    ("Tromso", "1990-01-15T10:00:00Z", 69.6492, 18.9553, "OWE"),
    ("Murmansk", "1984-06-21T00:30:00Z", 68.9585, 33.0827, "OWE"),
]

NAMES = {"P": "placidus", "W": "whole_sign", "E": "equal", "O": "porphyry"}


def main():
    out = {
        "source": "Swiss Ephemeris %s via pyswisseph, swe.houses_ex (tropical, geocentric)" % swe.version,
        "note": "Generated offline by scripts/astroref/houses_swisseph.py; polar charts use Porphyry (Placidus undefined).",
        "retrieved": datetime.date.today().isoformat(),
        "charts": [],
    }
    for name, instant, lat, lon, systems in CHARTS:
        t = datetime.datetime.strptime(instant, "%Y-%m-%dT%H:%M:%SZ")
        jd = swe.julday(t.year, t.month, t.day, t.hour + t.minute / 60 + t.second / 3600)
        chart = {"name": name, "instant": instant, "latitude": lat, "longitude": lon, "systems": {}}
        for s in systems:
            cusps, ascmc = swe.houses_ex(jd, lat, lon, s.encode())
            chart["systems"][NAMES[s]] = [round(c, 7) for c in cusps[:12]]
            chart["asc"] = round(ascmc[0], 7)
            chart["mc"] = round(ascmc[1], 7)
            chart["armc"] = round(ascmc[2], 7)
        out["charts"].append(chart)
    print(json.dumps(out, indent=2))


if __name__ == "__main__":
    main()
