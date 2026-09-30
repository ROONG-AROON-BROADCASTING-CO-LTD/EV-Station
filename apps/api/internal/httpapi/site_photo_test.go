package httpapi

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestOptimizeSitePhotoShrinksLargeJPEGWithoutChangingDimensions(t *testing.T) {
	photo := image.NewRGBA(image.Rect(0, 0, 1800, 1200))
	for y := 0; y < 1200; y++ {
		for x := 0; x < 1800; x++ {
			photo.SetRGBA(x, y, color.RGBA{R: uint8(x*17 + y*7), G: uint8(x*3 + y*13), B: uint8(x*11 + y*5), A: 255})
		}
	}
	var original bytes.Buffer
	if err := jpeg.Encode(&original, photo, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	if original.Len() < 1<<20 {
		t.Fatalf("test photo too small to exercise compression: %d bytes", original.Len())
	}
	optimized, err := optimizeSitePhoto("image/jpeg", original.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(optimized) >= original.Len() {
		t.Fatalf("photo was not made smaller: before=%d after=%d", original.Len(), len(optimized))
	}
	decoded, err := jpeg.DecodeConfig(bytes.NewReader(optimized))
	if err != nil || decoded.Width != 1800 || decoded.Height != 1200 {
		t.Fatalf("optimized photo is not a usable full-size JPEG: %+v, %v", decoded, err)
	}
	if unchanged, err := optimizeSitePhoto("application/pdf", original.Bytes()); err != nil || !bytes.Equal(unchanged, original.Bytes()) {
		t.Fatal("document bytes must be preserved")
	}
	// A valid APP1 EXIF segment must remain intact because it can specify
	// display orientation, which Go's JPEG encoder would otherwise discard.
	exif := []byte{0xff, 0xe1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0x00, 0x00}
	withExif := append(append(append([]byte{}, original.Bytes()[:2]...), exif...), original.Bytes()[2:]...)
	if unchanged, err := optimizeSitePhoto("image/jpeg", withExif); err != nil || !bytes.Equal(unchanged, withExif) {
		t.Fatal("EXIF camera photo must preserve its orientation metadata")
	}
}
