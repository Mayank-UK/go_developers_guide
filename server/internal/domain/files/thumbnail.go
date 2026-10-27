// Thumbnail path, stdlib only: image.DecodeConfig first (a cheap header
// read) checks the pixel budget BEFORE image.Decode allocates a single
// pixel row — the decode-bomb defense — then a hand-rolled box-sample
// downscale averages source boxes into destination pixels (no external
// imaging dependency; x/image is deliberately not pulled for this).
// Encoding stays stdlib too: png and jpeg, formats this package
// registers by importing.
package files

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
)

// MaxPixels is the decode budget: 24 million pixels (a 6000×4000
// photograph). A header claiming more is refused before Decode runs,
// because a 24-MP NRGBA image already reserves ~96 MB and decode bombs
// multiply that per request.
const MaxPixels = 24_000_000

// ErrImageTooLarge reports an image whose DECLARED dimensions exceed
// MaxPixels — refused from the header, before the pixel allocation.
var ErrImageTooLarge = errors.New("files: image dimensions exceed the decode budget")

// ThumbnailJPEGQuality is the teaching thumbnail's encode quality —
// 85 keeps text legible at a fraction of the bytes.
const ThumbnailJPEGQuality = 85

// DecodeBounded decodes an image of any registered format without
// trusting it: image.DecodeConfig reads only the header through a
// buffered view, the budget check refuses headers that claim more
// pixels than MaxPixels, and only then does image.Decode allocate. The
// seeker is rewound between the two passes because DecodeConfig
// CONSUMES the header from whatever reader it is handed — continuing
// on one shared stream would start the full decode mid-image (the
// classic gotcha this function exists to contain). r must therefore be
// a seeker; every real caller opens one over stored bytes, and the
// second pass re-reads only the tiny header prefix.
func DecodeBounded(r io.ReadSeeker) (image.Image, string, error) {
	config, format, err := image.DecodeConfig(bufio.NewReader(r))
	if err != nil {
		return nil, "", fmt.Errorf("files: read image header: %w", err)
	}
	if width, height := int64(config.Width), int64(config.Height); width*height > MaxPixels {
		return nil, "", fmt.Errorf("%w: %d×%d = %d pixels (budget %d)",
			ErrImageTooLarge, config.Width, config.Height, width*height, MaxPixels)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, "", fmt.Errorf("files: rewind image for decode: %w", err)
	}
	img, decoded, err := image.Decode(r)
	if err != nil {
		return nil, "", fmt.Errorf("files: decode %s image: %w", format, err)
	}
	return img, decoded, nil
}

// Thumbnail downscales img so its longest edge is at most maxDim, by
// box sampling: every destination pixel is the alpha-weighted average
// of the source box it covers, so solid regions survive exactly and
// edges blend rather than alias. Images already within maxDim are
// returned unchanged — inventing pixels (upscale) is not a thumbnail's
// job. maxDim < 1 is a programmer error and panics, per the
// conventions: callers derive it from validated input.
func Thumbnail(img image.Image, maxDim int) image.Image {
	if maxDim < 1 {
		panic(fmt.Sprintf("files: thumbnail maxDim %d must be >= 1", maxDim))
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := max(width, height)
	if longest <= maxDim {
		return img
	}
	dstW := max(1, width*maxDim/longest)
	dstH := max(1, height*maxDim/longest)
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))

	for y := 0; y < dstH; y++ {
		boxTop := bounds.Min.Y + y*height/dstH
		boxBottom := bounds.Min.Y + (y+1)*height/dstH
		if boxBottom <= boxTop {
			boxBottom = boxTop + 1
		}
		for x := 0; x < dstW; x++ {
			boxLeft := bounds.Min.X + x*width/dstW
			boxRight := bounds.Min.X + (x+1)*width/dstW
			if boxRight <= boxLeft {
				boxRight = boxLeft + 1
			}
			var sumR, sumG, sumB, sumA, samples uint64
			for sy := boxTop; sy < boxBottom && sy < bounds.Max.Y; sy++ {
				for sx := boxLeft; sx < boxRight && sx < bounds.Max.X; sx++ {
					c := color.NRGBAModel.Convert(img.At(sx, sy)).(color.NRGBA)
					// Weight color channels by alpha: transparent
					// pixels must not darken what shows through them.
					sumR += uint64(c.R) * uint64(c.A)
					sumG += uint64(c.G) * uint64(c.A)
					sumB += uint64(c.B) * uint64(c.A)
					sumA += uint64(c.A)
					samples++
				}
			}
			if sumA == 0 {
				dst.SetNRGBA(x, y, color.NRGBA{})
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(sumR / sumA),
				G: uint8(sumG / sumA),
				B: uint8(sumB / sumA),
				A: uint8(sumA / samples),
			})
		}
	}
	return dst
}

// EncodeThumbnail writes img in the named format — "png" lossless for
// graphics with sharp edges, "jpeg" for photographs and previews. Any
// other format is refused by name: an encoder switch must never fall
// through to a default that silently changes what the bytes mean.
func EncodeThumbnail(w io.Writer, img image.Image, format string) error {
	switch format {
	case "png":
		if err := png.Encode(w, img); err != nil {
			return fmt.Errorf("files: encode png thumbnail: %w", err)
		}
	case "jpeg":
		if err := jpeg.Encode(w, img, &jpeg.Options{Quality: ThumbnailJPEGQuality}); err != nil {
			return fmt.Errorf("files: encode jpeg thumbnail: %w", err)
		}
	default:
		return fmt.Errorf("files: thumbnail format %q not supported (png, jpeg)", format)
	}
	return nil
}
