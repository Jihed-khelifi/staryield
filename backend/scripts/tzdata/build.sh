#!/bin/sh
# Builds internal/astro/tz/zoneinfo.zip: the IANA time zone database compiled
# WITH the "backzone" data (PACKRATDATA=backzone PACKRATLIST=zone.tab), which
# restores pre-1970 history for zones that the main data merges into links
# (Europe/Oslo, Atlantic/Reykjavik, Europe/Amsterdam, Europe/Stockholm …).
# Birth charts need that history; Go's own embedded tzdata omits it.
#
# Usage (from backend/): sh scripts/tzdata/build.sh [version]
# Requires curl, tar, make, a C compiler and zip (all present on macOS with the
# Command Line Tools and on Debian/Ubuntu build images).
set -eu

VERSION="${1:-2026d}"
BASE="https://data.iana.org/time-zones/releases"
HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$HERE/../../internal/astro/tz/zoneinfo.zip"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cd "$WORK"
curl -fsSL -o tzcode.tar.gz "$BASE/tzcode$VERSION.tar.gz"
curl -fsSL -o tzdata.tar.gz "$BASE/tzdata$VERSION.tar.gz"
mkdir src
tar -xzf tzcode.tar.gz -C src
tar -xzf tzdata.tar.gz -C src
cd src
make -s PACKRATDATA=backzone PACKRATLIST=zone.tab TOPDIR="$WORK/root" ZFLAGS='-b slim' install >/dev/null

ZONEINFO="$WORK/root/usr/share/zoneinfo"
cd "$ZONEINFO"
# Keep only compiled zones: drop the posix/right variants, tables and helpers.
rm -rf posix right posixrules localtime Factory leapseconds leap-seconds.list \
	iso3166.tab zone.tab zone1970.tab zonenow.tab tzdata.zi SECURITY 2>/dev/null || true
printf '%s\n' "$VERSION" >+VERSION
rm -f "$OUT"
# Deterministic archive: sorted names, fixed timestamps, no extra attributes.
find . -type f | sed 's|^\./||' | LC_ALL=C sort >"$WORK/files.txt"
find . -exec touch -t 198001010000 {} +
zip -q -X -9 "$OUT" -@ <"$WORK/files.txt"
echo "wrote $OUT ($(wc -c <"$OUT") bytes, $(wc -l <"$WORK/files.txt") zones, tzdata $VERSION with backzone)"
