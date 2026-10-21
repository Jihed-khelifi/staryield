package service

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // register decoder (first frame of animations)
	"image/jpeg"
	"image/png"
	"net/http"

	xdraw "golang.org/x/image/draw"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/security"
)

// ImagePolicy bounds and normalises uploaded images.
type ImagePolicy struct {
	MaxBytes  int64
	MaxPixels int
	// MaxSide downscales the longest side to at most this many pixels.
	MaxSide int
	// AllowGIF accepts GIFs (only the first frame is kept).
	AllowGIF bool
	// KeepTransparency encodes images with transparent pixels as PNG
	// instead of flattening them into JPEG.
	KeepTransparency bool
	// Field names the form field in validation errors.
	Field string
}

// ProcessedImage is a re-encoded image ready for storage.
type ProcessedImage struct {
	Data        []byte
	ContentType string
	Width       int
	Height      int
}

// Extension returns the file extension for the content type.
func (p *ProcessedImage) Extension() string {
	if p.ContentType == "image/png" {
		return ".png"
	}
	return ".jpg"
}

// ProcessImage validates an upload by its content (never by the declared
// type or file name), guards against decompression bombs, then decodes and
// re-encodes it. Re-encoding drops EXIF/GPS metadata and anything smuggled
// in the original file.
func ProcessImage(data []byte, p ImagePolicy) (*ProcessedImage, error) {
	field := p.Field
	if field == "" {
		field = "image"
	}
	if int64(len(data)) > p.MaxBytes {
		return nil, apperror.PayloadTooLarge(fmt.Sprintf("Choose an image smaller than %d MB.", p.MaxBytes>>20))
	}
	if len(data) < 64 {
		return nil, apperror.Validation(map[string]string{field: "The file is not a valid image."})
	}
	ct := http.DetectContentType(data)
	switch ct {
	case "image/png", "image/jpeg", "image/webp":
	case "image/gif":
		if !p.AllowGIF {
			return nil, apperror.UnsupportedMedia("Use a PNG, JPEG or WebP image.")
		}
	default:
		if p.AllowGIF {
			return nil, apperror.UnsupportedMedia("Use a PNG, JPEG, WebP or GIF image.")
		}
		return nil, apperror.UnsupportedMedia("Use a PNG, JPEG or WebP image.")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, apperror.Validation(map[string]string{field: "The file is not a valid image."})
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 10000 || cfg.Height > 10000 || cfg.Width*cfg.Height > p.MaxPixels {
		return nil, apperror.Validation(map[string]string{field: "The image dimensions are too large."})
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, apperror.Validation(map[string]string{field: "The file is not a valid image."})
	}
	w, h := fit(src.Bounds().Dx(), src.Bounds().Dy(), p.MaxSide)
	transparent := p.KeepTransparency && hasTransparency(src)
	var dst draw.Image
	if transparent {
		dst = image.NewNRGBA(image.Rect(0, 0, w, h))
	} else {
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		// Flatten onto white so transparent areas do not turn black in JPEG.
		draw.Draw(rgba, rgba.Bounds(), image.White, image.Point{}, draw.Src)
		dst = rgba
	}
	op := xdraw.Over
	if transparent {
		op = xdraw.Src
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), op, nil)
	var buf bytes.Buffer
	out := &ProcessedImage{Width: w, Height: h}
	if transparent {
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, dst); err != nil {
			return nil, apperror.Internal(err)
		}
		out.ContentType = "image/png"
	} else {
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
			return nil, apperror.Internal(err)
		}
		out.ContentType = "image/jpeg"
	}
	out.Data = buf.Bytes()
	return out, nil
}

// fit scales (w, h) so the longest side is at most max, keeping the ratio.
func fit(w, h, max int) (int, int) {
	if max <= 0 || (w <= max && h <= max) {
		return w, h
	}
	if w >= h {
		return max, maxInt(1, h*max/w)
	}
	return maxInt(1, w*max/h), max
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// hasTransparency reports whether any pixel is not fully opaque.
func hasTransparency(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		return false
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
				return true
			}
		}
	}
	return false
}

// objectKey returns a random, unguessable storage key under prefix.
func objectKey(prefix, ext string) string {
	return prefix + "/" + security.MustRandomToken(18) + ext
}

func securityKeyed(secret, value string) string { return security.Keyed(secret, value, 32) }
