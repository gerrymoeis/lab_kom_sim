//go:build !android

package services

import (
	"image"
	"os"

	"github.com/gen2brain/heic"
)

// decodeImage decodes an image file, supporting HEIC/HEIF via gen2brain/heic
// and standard formats (JPEG, PNG) via Go's image.Decode with EXIF orientation.
func decodeImage(file *os.File, ext string) (image.Image, int, error) {
	if ext == ".heic" || ext == ".heif" {
		img, err := heic.Decode(file)
		if err != nil {
			return nil, 0, err
		}
		return img, 1, nil
	}

	orientation := readEXIFOrientation(file)
	if _, err := file.Seek(0, 0); err != nil {
		return nil, 0, err
	}
	img, _, err := image.Decode(file)
	return img, orientation, err
}
