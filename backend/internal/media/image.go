// Package media handles uploaded images: validation, re-encoding into a few
// widths, and storage in S3-compatible object storage.
package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder
)

const (
	MaxUploadBytes = 10 << 20 // 10 MB
	maxSide        = 10_000
	maxPixels      = 40_000_000 // decompression-bomb guard
	jpegQuality    = 82
)

// Widths are the sizes served to browsers (responsive images, brief 14.4).
var Widths = []int{480, 960, 1600}

var (
	ErrUnsupported = errors.New("use a JPG, PNG or WebP image")
	ErrTooBig      = errors.New("the image is too large; use one under 10 MB and 10,000 pixels on each side")
	ErrCorrupt     = errors.New("the image could not be read; it may be damaged")
)

type Variant struct {
	Width, Height int
	Data          []byte
}

type Processed struct {
	ContentType string
	Ext         string
	Variants    []Variant // smallest first; the last is the largest
}

// Process checks an upload is really an image (by content, not file name),
// fixes phone-camera rotation, and re-encodes it into Widths. Re-encoding
// drops all metadata, including GPS location.
func Process(data []byte) (*Processed, error) { return process(data, Widths) }

// DocumentImageWidth is the widest a photo of a document is kept: enough to
// read small print, without the full camera resolution.
const DocumentImageWidth = 2400

// ProcessDocumentImage re-encodes a photo or scan of a document once, at up
// to DocumentImageWidth, upright and without metadata (no GPS location).
func ProcessDocumentImage(data []byte) (contentType, ext string, out []byte, err error) {
	p, err := process(data, []int{DocumentImageWidth})
	if err != nil {
		return "", "", nil, err
	}
	return p.ContentType, p.Ext, p.Variants[len(p.Variants)-1].Data, nil
}

func process(data []byte, targetWidths []int) (*Processed, error) {
	if len(data) == 0 {
		return nil, ErrCorrupt
	}
	if len(data) > MaxUploadBytes {
		return nil, ErrTooBig
	}
	sniffed := http.DetectContentType(data)
	if sniffed != "image/jpeg" && sniffed != "image/png" && sniffed != "image/webp" {
		return nil, ErrUnsupported
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrCorrupt
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxSide || cfg.Height > maxSide || cfg.Width*cfg.Height > maxPixels {
		return nil, ErrTooBig
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrCorrupt
	}
	if format == "jpeg" {
		img = applyOrientation(img, jpegOrientation(data))
	}

	out := &Processed{ContentType: "image/jpeg", Ext: "jpg"}
	if format == "png" {
		out.ContentType, out.Ext = "image/png", "png"
	}

	b := img.Bounds()
	largest := min(b.Dx(), targetWidths[len(targetWidths)-1])
	var widths []int
	for _, w := range targetWidths {
		if w < largest {
			widths = append(widths, w)
		}
	}
	widths = append(widths, largest)

	for _, w := range widths {
		h := max(1, b.Dy()*w/b.Dx())
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		if out.Ext == "jpg" {
			// JPEG has no transparency: flatten onto white, not black.
			draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		}
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)

		var buf bytes.Buffer
		if out.Ext == "png" {
			err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, dst)
		} else {
			err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality})
		}
		if err != nil {
			return nil, err
		}
		out.Variants = append(out.Variants, Variant{Width: w, Height: h, Data: buf.Bytes()})
	}
	return out, nil
}

// jpegOrientation reads the EXIF orientation tag (1 to 8) from a JPEG, or 1.
func jpegOrientation(data []byte) int {
	r := data
	if len(r) < 4 || r[0] != 0xFF || r[1] != 0xD8 {
		return 1
	}
	r = r[2:]
	for len(r) >= 4 && r[0] == 0xFF {
		marker := r[1]
		size := int(binary.BigEndian.Uint16(r[2:4]))
		if size < 2 || len(r) < 2+size {
			return 1
		}
		seg := r[4 : 2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		if marker == 0xDA { // start of scan: no more metadata
			return 1
		}
		r = r[2+size:]
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:8]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off : off+2]))
	for i := 0; i < n; i++ {
		e := off + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8 : e+10])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation rotates or flips img so it displays upright.
func applyOrientation(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	swap := o >= 5
	dw, dh := w, h
	if swap {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.Set(nx, ny, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
