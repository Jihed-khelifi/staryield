// Package astro provides the astronomical and astrological building blocks
// used by Staryield: a compact low-precision ephemeris, natal chart
// construction (planets, Ascendant, Midheaven and equal houses), lunar phases,
// retrograde periods, a small embedded world gazetteer, numerology helpers and
// a deterministic daily horoscope generator.
//
// The ephemeris follows Paul Schlyter's "How to compute planetary positions"
// method (Keplerian orbital elements referred to the ecliptic and equinox of
// date plus the main lunar and Jupiter/Saturn/Uranus perturbations). It is
// accurate to a few arcminutes for the Sun, Moon and planets between roughly
// 1900 and 2100, which is far finer than the one-degree resolution that
// astrological interpretation relies on.
//
// Everything in this package is pure computation on the standard library and
// is safe for concurrent use. Generated text is offered as reflection and
// entertainment: it describes perspective, never destiny.
package astro
