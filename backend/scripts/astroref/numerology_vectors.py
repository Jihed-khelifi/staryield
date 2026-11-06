"""Write the shared numerology fixtures (backend Go tests and frontend TS tests).

An independent implementation of the documented formulas:
  life path      = R(R(month) + R(day) + R(year)), keeping 11, 22, 33 at every step
  personal year  = R(R(birth month) + R(birth day) + R(year of as_of)), 1–9
  personal month = R(personal year + month of as_of), 1–9
  personal day   = R(personal month + day of as_of), 1–9
where R sums decimal digits until one digit remains (or a master number when
allowed). "as_of" is the calendar date in the requester's time zone.

    python3 scripts/astroref/numerology_vectors.py > internal/astro/testdata/numerology_vectors.json
"""

import datetime
import json
from zoneinfo import ZoneInfo

MASTERS = (11, 22, 33)


def reduce(n, keep):
    while n > 9 and not (keep and n in MASTERS):
        n = sum(int(c) for c in str(n))
    return n


def life_path(d):
    return reduce(reduce(d.month, True) + reduce(d.day, True) + reduce(d.year, True), True)


def personal(d, as_of):
    py = reduce(reduce(d.month, False) + reduce(d.day, False) + reduce(as_of.year, False), False)
    pm = reduce(py + as_of.month, False)
    pd = reduce(pm + as_of.day, False)
    return py, pm, pd


def vector(birth, as_of, note=""):
    lp = life_path(birth)
    py, pm, pd = personal(birth, as_of)
    v = {
        "birth_date": birth.isoformat(),
        "as_of": as_of.isoformat(),
        "life_path": lp,
        "life_path_master": lp in MASTERS,
        "personal_year": py,
        "personal_month": pm,
        "personal_day": pd,
    }
    if note:
        v["note"] = note
    return v


def main():
    as_of = datetime.date(2026, 11, 4)
    births = [
        (datetime.date(1990, 11, 29), "worked example: 11 + 11 + 1 = 23 -> 5"),
        (datetime.date(1987, 11, 29), "master 11"),
        (datetime.date(1980, 1, 1), "component method keeps 11 (digit sum gives 2)"),
        (datetime.date(1945, 4, 30), ""),
        (datetime.date(2000, 2, 29), "leap day"),
        (datetime.date(1999, 12, 31), ""),
        (datetime.date(1900, 1, 1), "earliest supported year"),
        (datetime.date(2026, 11, 4), "born today"),
    ]
    vectors = [vector(b, as_of, n) for b, n in births]
    # Add every master life path found in a deterministic scan, plus a spread
    # of ordinary dates, until at least 48 vectors exist.
    found = {11: 0, 22: 0, 33: 0}
    d = datetime.date(1920, 1, 7)
    while len(vectors) < 48 and d.year < 2026:
        lp = life_path(d)
        if lp in found and found[lp] < 4:
            found[lp] += 1
            vectors.append(vector(d, as_of, "master %d" % lp))
        elif d.day % 11 == 3 and len(vectors) < 40:
            vectors.append(vector(d, datetime.date(2026, (d.month % 12) + 1, min(d.day, 28))))
        d += datetime.timedelta(days=97)
    tz_vectors = []
    for birth, instant, zone in [
        (datetime.date(1987, 11, 29), "2026-12-31T23:30:00Z", "Asia/Tokyo"),
        (datetime.date(1987, 11, 29), "2027-01-01T05:00:00Z", "America/Los_Angeles"),
        (datetime.date(1990, 11, 29), "2026-11-30T10:30:00Z", "Pacific/Kiritimati"),
        (datetime.date(1990, 11, 29), "2026-11-30T10:30:00Z", "Pacific/Pago_Pago"),
    ]:
        t = datetime.datetime.strptime(instant, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc)
        local = t.astimezone(ZoneInfo(zone)).date()
        v = vector(birth, local, "as_of is the calendar date in %s" % zone)
        v["instant"] = instant
        v["tz"] = zone
        tz_vectors.append(v)
    out = {
        "description": "Shared numerology fixtures for the Go backend and the TypeScript frontend.",
        "method": "Component reduction: life path keeps 11/22/33 at every step; personal cycles reduce to 1-9.",
        "vectors": vectors,
        "timezone_vectors": tz_vectors,
    }
    print(json.dumps(out, indent=2))


if __name__ == "__main__":
    main()
