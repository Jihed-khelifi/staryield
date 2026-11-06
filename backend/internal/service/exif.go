package service

import (
	"encoding/binary"
	"image"
	"image/draw"
)

// exifOrientationTag is the TIFF/EXIF Orientation tag.
const exifOrientationTag = 0x0112

// jpegOrientation returns the EXIF orientation (1–8) stored in a JPEG, or 1
// when there is none or the metadata is malformed. Phone cameras store the
// pixels as captured and record the rotation here; because ProcessImage
// re-encodes without metadata, the rotation must be applied to the pixels or
// portrait photos come out sideways.
//
// The parser only walks segment headers and one IFD with strict bounds
// checks; it never allocates based on values read from the file.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return 1
		}
		marker := data[pos+1]
		switch {
		case marker == 0xFF: // fill byte
			pos++
			continue
		case marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01:
			pos += 2 // markers without a length
			continue
		case marker == 0xDA || marker == 0xD9: // start of scan / end of image
			return 1
		}
		length := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if length < 2 || pos+2+length > len(data) {
			return 1
		}
		segment := data[pos+4 : pos+2+length]
		if marker == 0xE1 && len(segment) >= 6 && string(segment[:6]) == "Exif\x00\x00" {
			return tiffOrientation(segment[6:])
		}
		pos += 2 + length
	}
	return 1
}

// tiffOrientation reads the Orientation tag from IFD0 of a TIFF header.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 1
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 1
	}
	count := int(order.Uint16(tiff[ifd : ifd+2]))
	for i := 0; i < count; i++ {
		entry := ifd + 2 + i*12
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:entry+2]) != exifOrientationTag {
			continue
		}
		// Type SHORT (3), count 1: the value sits in the first two bytes of
		// the value field.
		if order.Uint16(tiff[entry+2:entry+4]) != 3 || order.Uint32(tiff[entry+4:entry+8]) != 1 {
			return 1
		}
		if v := int(order.Uint16(tiff[entry+8 : entry+10])); v >= 1 && v <= 8 {
			return v
		}
		return 1
	}
	return 1
}

// applyOrientation returns img transformed so it displays upright for the
// given EXIF orientation. Orientations 5–8 swap width and height.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	src := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch orientation {
			case 2: // mirrored horizontally
				sx, sy = w-1-x, y
			case 3: // rotated 180°
				sx, sy = w-1-x, h-1-y
			case 4: // mirrored vertically
				sx, sy = x, h-1-y
			case 5: // mirrored along the top-left diagonal
				sx, sy = y, x
			case 6: // stored rotated 90° counter-clockwise: rotate clockwise
				sx, sy = y, h-1-x
			case 7: // mirrored along the top-right diagonal
				sx, sy = w-1-y, h-1-x
			case 8: // stored rotated 90° clockwise: rotate counter-clockwise
				sx, sy = w-1-y, x
			}
			si := src.PixOffset(sx, sy)
			di := dst.PixOffset(x, y)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}
