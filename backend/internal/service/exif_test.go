package service

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// exifSegment builds an APP1 segment holding only an Orientation tag.
func exifSegment(order binary.ByteOrder, orientation uint16) []byte {
	var tiff bytes.Buffer
	if order == binary.ByteOrder(binary.LittleEndian) {
		tiff.WriteString("II")
	} else {
		tiff.WriteString("MM")
	}
	for _, v := range []any{uint16(42), uint32(8), uint16(1), uint16(exifOrientationTag), uint16(3), uint32(1), orientation, uint16(0), uint32(0)} {
		_ = binary.Write(&tiff, order, v)
	}
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}

// withSegment inserts a segment right after the JPEG SOI marker.
func withSegment(jpg, seg []byte) []byte {
	out := append([]byte{0xFF, 0xD8}, seg...)
	return append(out, jpg[2:]...)
}

// leftRedRightBlue is a w×h JPEG: left half red, right half blue.
func leftRedRightBlue(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{R: 220, A: 255}
			if x >= w/2 {
				c = color.RGBA{B: 220, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0x9000 && g < 0x6000 && b < 0x6000
}

func isBlue(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return b > 0x9000 && r < 0x6000 && g < 0x6000
}

func TestJPEGOrientationParser(t *testing.T) {
	base := leftRedRightBlue(t, 8, 8)
	if got := jpegOrientation(base); got != 1 {
		t.Fatalf("no EXIF: %d", got)
	}
	for o := uint16(1); o <= 8; o++ {
		for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
			if got := jpegOrientation(withSegment(base, exifSegment(order, o))); got != int(o) {
				t.Fatalf("orientation %d (%v): got %d", o, order, got)
			}
		}
	}
	// Out-of-range values and garbage fall back to 1 without panicking.
	if got := jpegOrientation(withSegment(base, exifSegment(binary.BigEndian, 9))); got != 1 {
		t.Fatalf("orientation 9: %d", got)
	}
	for _, junk := range [][]byte{nil, {0xFF}, {0xFF, 0xD8}, {0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}, []byte("not a jpeg at all")} {
		if got := jpegOrientation(junk); got != 1 {
			t.Fatalf("junk %x: %d", junk, got)
		}
	}
}

func TestProcessImageAppliesEXIFOrientation(t *testing.T) {
	base := leftRedRightBlue(t, 80, 40) // landscape: red on the left
	policy := ImagePolicy{MaxBytes: 1 << 20, MaxPixels: 1 << 20, MaxSide: 1000, Field: "image"}
	cases := []struct {
		orientation uint16
		w, h        int
		redAt       image.Point // a point that must be red after correction
		blueAt      image.Point
	}{
		{1, 80, 40, image.Pt(10, 20), image.Pt(70, 20)},
		{2, 80, 40, image.Pt(70, 20), image.Pt(10, 20)}, // mirrored: red moves right
		{3, 80, 40, image.Pt(70, 20), image.Pt(10, 20)}, // 180°
		{4, 80, 40, image.Pt(10, 20), image.Pt(70, 20)}, // vertical flip keeps left/right
		{5, 40, 80, image.Pt(20, 10), image.Pt(20, 70)}, // transpose: left → top
		{6, 40, 80, image.Pt(20, 10), image.Pt(20, 70)}, // rotate 90° CW: left → top
		{7, 40, 80, image.Pt(20, 70), image.Pt(20, 10)}, // transverse: left → bottom
		{8, 40, 80, image.Pt(20, 70), image.Pt(20, 10)}, // rotate 90° CCW: left → bottom
	}
	for _, tc := range cases {
		out, err := ProcessImage(withSegment(base, exifSegment(binary.BigEndian, tc.orientation)), policy)
		if err != nil {
			t.Fatalf("orientation %d: %v", tc.orientation, err)
		}
		if out.Width != tc.w || out.Height != tc.h {
			t.Fatalf("orientation %d: size %dx%d, want %dx%d", tc.orientation, out.Width, out.Height, tc.w, tc.h)
		}
		img, err := jpeg.Decode(bytes.NewReader(out.Data))
		if err != nil {
			t.Fatal(err)
		}
		if !isRed(img.At(tc.redAt.X, tc.redAt.Y)) || !isBlue(img.At(tc.blueAt.X, tc.blueAt.Y)) {
			t.Fatalf("orientation %d: red@%v=%v blue@%v=%v", tc.orientation, tc.redAt, img.At(tc.redAt.X, tc.redAt.Y),
				tc.blueAt, img.At(tc.blueAt.X, tc.blueAt.Y))
		}
		// The output never carries EXIF again (no double rotation in browsers).
		if jpegOrientation(out.Data) != 1 || bytes.Contains(out.Data, []byte("Exif\x00\x00")) {
			t.Fatalf("orientation %d: output still has EXIF", tc.orientation)
		}
	}
}

func FuzzJPEGOrientation(f *testing.F) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	f.Add(buf.Bytes())
	f.Add(withSegment(buf.Bytes(), exifSegment(binary.LittleEndian, 6)))
	f.Add(withSegment(buf.Bytes(), exifSegment(binary.BigEndian, 8)))
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x10, 'E', 'x', 'i', 'f', 0, 0, 'M', 'M', 0, 42, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, data []byte) {
		if o := jpegOrientation(data); o < 1 || o > 8 {
			t.Fatalf("orientation out of range: %d", o)
		}
	})
}
