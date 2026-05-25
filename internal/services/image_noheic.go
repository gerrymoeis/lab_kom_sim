//go:build android

package services

import (
	"fmt"
	"image"
	"os"
)

// decodeImage decodes an image file, supporting standard formats only (JPEG, PNG).
// HEIC/HEIF files are rejected since CGO is not available on Android builds.
func decodeImage(file *os.File, ext string) (image.Image, int, error) {
	if ext == ".heic" || ext == ".heif" {
		return nil, 0, fmt.Errorf("format HEIC tidak didukung di perangkat ini")
	}

	orientation := readEXIFOrientation(file)
	if _, err := file.Seek(0, 0); err != nil {
		return nil, 0, err
	}
	img, _, err := image.Decode(file)
	return img, orientation, err
}
