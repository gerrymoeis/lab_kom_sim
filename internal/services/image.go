package services

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

// ImageService handles image compression and processing
type ImageService struct {
	maxWidth  int
	maxHeight int
	quality   int
}

// NewImageService creates a new ImageService with default settings
func NewImageService() *ImageService {
	return &ImageService{
		maxWidth:  1920,
		maxHeight: 1920,
		quality:   75,
	}
}

// CompressAndSave compresses an image and saves it to the destination path.
// Supports JPEG, PNG, and HEIC formats (HEIC only on !android builds).
// maxDimension: maximum width or height (maintains aspect ratio).
func (s *ImageService) CompressAndSave(sourcePath, destPath string, maxDimension int) error {
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer srcFile.Close()

	ext := strings.ToLower(filepath.Ext(sourcePath))

	img, orientation, err := decodeImage(srcFile, ext)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	img = s.applyOrientation(img, orientation)

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var resized image.Image
	if width > maxDimension || height > maxDimension {
		resized = imaging.Fit(img, maxDimension, maxDimension, imaging.MitchellNetravali)
	} else {
		resized = img
	}

	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	options := &jpeg.Options{Quality: s.quality}
	if err := jpeg.Encode(destFile, resized, options); err != nil {
		return fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return nil
}

// readEXIFOrientation extracts EXIF orientation from JPEG/PNG file
func readEXIFOrientation(file *os.File) int {
	x, err := exif.Decode(file)
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	orient, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return orient
}

// applyOrientation applies EXIF orientation transformation to image
func (s *ImageService) applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 1:
		return img
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.FlipH(imaging.Rotate270(img))
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.FlipH(imaging.Rotate90(img))
	case 8:
		return imaging.Rotate90(img)
	default:
		return img
	}
}

// DeleteImage deletes an image file if it exists
func (s *ImageService) DeleteImage(filePath string) error {
	if filePath == "" {
		return nil
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}
	return nil
}

// GetImageSize returns the size of an image file in bytes
func (s *ImageService) GetImageSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	return info.Size(), nil
}
