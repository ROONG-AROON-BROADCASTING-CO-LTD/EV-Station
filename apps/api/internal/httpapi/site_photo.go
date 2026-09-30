package httpapi

import (
	"bytes"
	"fmt"
	"image/jpeg"
)

// optimizeSitePhoto only changes photos explicitly classified as photos by
// the client. Documents (including photographed deeds) retain their bytes.
// JPEG is recompressed only when the result is smaller; PNG and WebP are kept
// intact because changing either can remove transparency or increase size.
func optimizeSitePhoto(mimeType string, data []byte) ([]byte, error) {
	if mimeType != "image/jpeg" || len(data) < 1<<20 {
		return data, nil
	}
	// JPEG re-encoding drops EXIF orientation. Keep camera originals until an
	// orientation-aware encoder is available; otherwise portrait photos rotate.
	if bytes.Contains(data, []byte("Exif\x00\x00")) {
		return data, nil
	}
	configuration, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || configuration.Width <= 0 || configuration.Height <= 0 || int64(configuration.Width)*int64(configuration.Height) > 50_000_000 {
		return nil, fmt.Errorf("invalid or oversized site photo")
	}
	photo, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid site photo: %w", err)
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, photo, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	if output.Len() >= len(data) {
		return data, nil
	}
	return output.Bytes(), nil
}
