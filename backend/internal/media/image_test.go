package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestProcessMakesWidthsNoWiderThanOriginal(t *testing.T) {
	p, err := Process(encodePNG(t, 2400, 1200))
	if err != nil {
		t.Fatal(err)
	}
	var widths []int
	for _, v := range p.Variants {
		widths = append(widths, v.Width)
		img, err := png.Decode(bytes.NewReader(v.Data))
		if err != nil || img.Bounds().Dx() != v.Width || img.Bounds().Dy() != v.Height {
			t.Fatalf("variant %d does not decode to its size: %v", v.Width, err)
		}
	}
	if len(widths) != 3 || widths[0] != 480 || widths[2] != 1600 || p.Variants[2].Height != 800 {
		t.Fatalf("widths %v, largest height %d", widths, p.Variants[2].Height)
	}

	small, err := Process(encodePNG(t, 300, 200))
	if err != nil || len(small.Variants) != 1 || small.Variants[0].Width != 300 {
		t.Fatalf("small image should keep its size and not be upscaled: %+v %v", small, err)
	}
}

func TestProcessRejectsNonImages(t *testing.T) {
	cases := map[string][]byte{
		"text":      []byte("hello, this is not an image at all"),
		"html":      []byte("<html><script>alert(1)</script></html>"),
		"truncated": encodePNG(t, 50, 50)[:60],
		"empty":     {},
	}
	for name, data := range cases {
		if _, err := Process(data); !errors.Is(err, ErrUnsupported) && !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

// A tiny file can claim enormous dimensions; it must be refused before
// decoding allocates memory for it.
func TestProcessRejectsDecompressionBombs(t *testing.T) {
	data := encodePNG(t, 1, 1)
	// IHDR data starts at byte 16: width, height (big endian), then CRC at 29.
	binary.BigEndian.PutUint32(data[16:20], 20000)
	binary.BigEndian.PutUint32(data[20:24], 20000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	if _, err := Process(data); !errors.Is(err, ErrTooBig) {
		t.Fatalf("err = %v, want ErrTooBig", err)
	}
}

// jpegWithOrientation builds a w×h JPEG carrying an EXIF orientation tag and
// a fake GPS marker string inside the EXIF block.
func jpegWithOrientation(t *testing.T, w, h int, orientation uint16) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	raw := b.Bytes()

	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08")
	ifd := make([]byte, 2+12+4)
	binary.BigEndian.PutUint16(ifd[0:2], 1)
	binary.BigEndian.PutUint16(ifd[2:4], 0x0112)
	binary.BigEndian.PutUint16(ifd[4:6], 3)
	binary.BigEndian.PutUint32(ifd[6:10], 1)
	binary.BigEndian.PutUint16(ifd[10:12], orientation)
	payload := append([]byte("Exif\x00\x00"), append(append(tiff, ifd...), []byte("GPS-SECRET-LOCATION")...)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:4], uint16(len(payload)+2))
	seg = append(seg, payload...)
	return append(append([]byte{0xFF, 0xD8}, seg...), raw[2:]...)
}

func TestProcessAppliesOrientationAndStripsMetadata(t *testing.T) {
	data := jpegWithOrientation(t, 40, 20, 6) // phone photo held upright
	if jpegOrientation(data) != 6 {
		t.Fatal("test JPEG has no orientation tag")
	}
	p, err := Process(data)
	if err != nil {
		t.Fatal(err)
	}
	v := p.Variants[len(p.Variants)-1]
	if v.Width != 20 || v.Height != 40 {
		t.Fatalf("output %dx%d, want 20x40 (rotated upright)", v.Width, v.Height)
	}
	if bytes.Contains(v.Data, []byte("Exif")) || bytes.Contains(v.Data, []byte("GPS-SECRET")) {
		t.Fatal("output still carries EXIF metadata")
	}
	if p.ContentType != "image/jpeg" {
		t.Fatalf("content type %s", p.ContentType)
	}
}
