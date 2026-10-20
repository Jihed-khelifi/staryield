package repository

import (
	"strings"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 20, 12, 0, 0, 123000, time.UTC)
	c, err := DecodeCursor(EncodeCursor(at, "01a159dd-92e4-7478-8a27-442bfecc6977"))
	if err != nil || !c.At.Equal(at) || c.ID != "01a159dd-92e4-7478-8a27-442bfecc6977" {
		t.Fatalf("roundtrip: %+v %v", c, err)
	}
	if c, err := DecodeCursor(""); c != nil || err != nil {
		t.Fatal("empty cursor")
	}
	for _, bad := range []string{"%%%", "e30", strings.Repeat("a", 300)} {
		if _, err := DecodeCursor(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if escapeLike(`50%_\`) != `50\%\_\\` {
		t.Fatal("LIKE escaping")
	}
}
