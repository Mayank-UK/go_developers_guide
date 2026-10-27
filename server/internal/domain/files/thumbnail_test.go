package files

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

// The thumbnail path: box sampling preserves solid regions EXACTLY
// (each destination pixel averages one uniform source box), the budget
// refuses decode bombs from the header alone, and both encodings
// round-trip through the stdlib codecs.

func solidImage(width, height int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func TestThumbnailDownscalesSolidRegionsExactly(t *testing.T) {
	// A 64×32 image of four solid quadrants, downscaled to longest edge
	// 32 → 32×16: each destination pixel averages ONE quadrant's color,
	// so the quadrant colors must survive bit-for-bit at the centers.
	img := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	quadrants := [2][2]color.NRGBA{
		{{R: 0xFF, A: 0xFF}, {G: 0xFF, A: 0xFF}},
		{{B: 0xFF, A: 0xFF}, {R: 0x80, G: 0x40, A: 0xFF}},
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, quadrants[y/16][x/32])
		}
	}
	thumb := Thumbnail(img, 32)
	bounds := thumb.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 16 {
		t.Fatalf("thumb = %d×%d, want 32×16 (aspect preserved)", bounds.Dx(), bounds.Dy())
	}
	for _, tc := range []struct {
		x, y int
		want color.NRGBA
	}{
		{8, 4, quadrants[0][0]},
		{24, 4, quadrants[0][1]},
		{8, 12, quadrants[1][0]},
		{24, 12, quadrants[1][1]},
	} {
		got := color.NRGBAModel.Convert(thumb.At(tc.x, tc.y)).(color.NRGBA)
		if got != tc.want {
			t.Errorf("thumb(%d,%d) = %+v, want the exact quadrant %+v", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestThumbnailLeavesSmallImagesAlone(t *testing.T) {
	img := solidImage(8, 4, color.NRGBA{R: 1, G: 2, B: 3, A: 0xFF})
	if got := Thumbnail(img, 64); got != image.Image(img) {
		t.Error("an image already within maxDim must be returned unchanged, never upscaled")
	}
}

func TestThumbnailWeightsByAlpha(t *testing.T) {
	// 62 of 64 columns are opaque red, the last 2 fully transparent.
	// Downscaled 64→16 (each dst pixel covers 4 columns), the rightmost
	// dst pixel averages 2 red + 2 transparent: alpha-WEIGHTED, its red
	// stays 255; a naive unweighted average would darken it to ~127.
	img := image.NewNRGBA(image.Rect(0, 0, 64, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 64; x++ {
			if x < 62 {
				img.SetNRGBA(x, y, color.NRGBA{R: 0xFF, A: 0xFF})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{})
			}
		}
	}
	thumb := Thumbnail(img, 16)
	got := color.NRGBAModel.Convert(thumb.At(15, 0)).(color.NRGBA)
	if got.R != 0xFF {
		t.Errorf("edge pixel red = %d, want 255 (transparent neighbors carry no color weight)", got.R)
	}
	if got.A < 100 || got.A > 155 {
		t.Errorf("edge pixel alpha = %d, want ~127 (half the box covered)", got.A)
	}
}

func TestThumbnailPanicsOnImpossibleMaxDim(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("maxDim < 1 is a programmer error and must panic")
		}
	}()
	Thumbnail(solidImage(4, 4, color.NRGBA{A: 0xFF}), 0)
}

// bombPNG crafts a valid PNG header claiming width×height — the shape
// of a real decode bomb: a few dozen bytes that would allocate
// gigabytes if decoded.
func bombPNG(width, height int) []byte {
	out := bytes.NewBuffer(nil)
	out.Write([]byte("\x89PNG\r\n\x1a\n"))
	chunk := bytes.NewBuffer(nil)
	chunk.Write([]byte("IHDR"))
	binary.Write(chunk, binary.BigEndian, uint32(width))
	binary.Write(chunk, binary.BigEndian, uint32(height))
	chunk.Write([]byte{8, 2, 0, 0, 0}) // 8-bit truecolor
	binary.Write(out, binary.BigEndian, uint32(13))
	out.Write(chunk.Bytes())
	binary.Write(out, binary.BigEndian, crc32.ChecksumIEEE(chunk.Bytes()))
	return out.Bytes()
}

func TestDecodeBoundedRefusesBombsFromTheHeader(t *testing.T) {
	bomb := bombPNG(30000, 30000) // 900M pixels in 33 bytes
	_, _, err := DecodeBounded(bytes.NewReader(bomb))
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("err = %v, want ErrImageTooLarge refused BEFORE decoding", err)
	}
	// And the refusal is the BUDGET's, not the truncated body's: an
	// in-budget header of the same truncated shape fails differently
	// (image: format error from the missing pixel data), proving the
	// budget check is what fired above.
	_, _, truncErr := DecodeBounded(bytes.NewReader(bombPNG(60, 40)))
	if errors.Is(truncErr, ErrImageTooLarge) {
		t.Errorf("an in-budget header must not trip the budget: %v", truncErr)
	}
}

func TestDecodeBoundedRefusesGarbage(t *testing.T) {
	if _, _, err := DecodeBounded(bytes.NewReader([]byte("not an image at all"))); err == nil {
		t.Error("garbage must not decode")
	}
}

func TestDecodeEncodeRoundTrip(t *testing.T) {
	source := solidImage(64, 32, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xFF})
	pngBuf := bytes.Buffer{}
	if err := png.Encode(&pngBuf, source); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	decoded, format, err := DecodeBounded(bytes.NewReader(pngBuf.Bytes()))
	if err != nil || format != "png" {
		t.Fatalf("decode png: format=%q err=%v", format, err)
	}
	thumb := Thumbnail(decoded, 16)
	jpegBuf := bytes.Buffer{}
	if err := EncodeThumbnail(&jpegBuf, thumb, "jpeg"); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	back, err := jpeg.Decode(bytes.NewReader(jpegBuf.Bytes()))
	if err != nil {
		t.Fatalf("decode jpeg: %v", err)
	}
	if back.Bounds().Dx() != 16 || back.Bounds().Dy() != 8 {
		t.Errorf("jpeg round-trip = %d×%d, want 16×8", back.Bounds().Dx(), back.Bounds().Dy())
	}
}

func TestEncodeThumbnailRefusesUnknownFormatsByName(t *testing.T) {
	img := solidImage(4, 4, color.NRGBA{A: 0xFF})
	if err := EncodeThumbnail(&bytes.Buffer{}, img, "gif"); err == nil {
		t.Error("an unknown format must be refused, never silently re-encoded")
	}
}
