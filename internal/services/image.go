package services

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/heic"
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
		quality:   75, // JPEG quality 75% for good balance
	}
}

// CompressAndSave compresses an image and saves it to the destination path
// Supports JPEG, PNG, and HEIC formats with automatic EXIF orientation correction
// maxDimension: maximum width or height (maintains aspect ratio)
// Returns error if compression fails
func (s *ImageService) CompressAndSave(sourcePath, destPath string, maxDimension int) error {
	tStart := time.Now()

	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer srcFile.Close()

	ext := strings.ToLower(filepath.Ext(sourcePath))
	var img image.Image
	var orientation int = 1

	log.Printf("[ImageService] Processing file: %s (ext: %s)", sourcePath, ext)

	if ext == ".heic" || ext == ".heif" {
		t0 := time.Now()
		img, err = heic.Decode(srcFile)
		if err != nil {
			return fmt.Errorf("failed to decode HEIC image: %w", err)
		}
		log.Printf("[ImageService] HEIC decode: %v, dims: %dx%d", time.Since(t0), img.Bounds().Dx(), img.Bounds().Dy())

		orientation = 1
	} else {
		orientation = s.getOrientationFromFile(srcFile)
		log.Printf("[ImageService] JPEG/PNG orientation detected: %d", orientation)

		srcFile.Seek(0, 0)

		t0 := time.Now()
		img, _, err = image.Decode(srcFile)
		if err != nil {
			return fmt.Errorf("failed to decode image: %w", err)
		}
		log.Printf("[ImageService] JPEG/PNG decode: %v, dims: %dx%d", time.Since(t0), img.Bounds().Dx(), img.Bounds().Dy())
	}

	t2 := time.Now()
	log.Printf("[ImageService] Applying orientation transformation: %d", orientation)
	img = s.applyOrientation(img, orientation)
	log.Printf("[ImageService] Orientation transform: %v, new dims: %dx%d", time.Since(t2), img.Bounds().Dx(), img.Bounds().Dy())

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var resized image.Image
	if width > maxDimension || height > maxDimension {
		t3 := time.Now()
		resized = imaging.Fit(img, maxDimension, maxDimension, imaging.MitchellNetravali)
		log.Printf("[ImageService] Resize (MitchellNetravali): %v, dims: %dx%d", time.Since(t3), resized.Bounds().Dx(), resized.Bounds().Dy())
	} else {
		resized = img
		log.Printf("[ImageService] No resize needed (within %d)", maxDimension)
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

	t4 := time.Now()
	options := &jpeg.Options{Quality: s.quality}
	if err := jpeg.Encode(destFile, resized, options); err != nil {
		return fmt.Errorf("failed to encode JPEG: %w", err)
	}
	log.Printf("[ImageService] JPEG encode: %v", time.Since(t4))

	log.Printf("[ImageService] TOTAL: %v — saved to: %s", time.Since(tStart), destPath)
	return nil
}

// getOrientationFromFile extracts EXIF orientation from JPEG/PNG file
func (s *ImageService) getOrientationFromFile(file *os.File) int {
	// Try to decode EXIF
	x, err := exif.Decode(file)
	if err != nil {
		return 1 // Default orientation if EXIF not found
	}

	// Get orientation tag
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1 // Default orientation if tag not found
	}

	orient, err := tag.Int(0)
	if err != nil {
		return 1
	}

	return orient
}

// getOrientationFromExif extracts orientation from EXIF byte data
func (s *ImageService) getOrientationFromExif(exifData []byte) int {
	
	// Create a reader from the EXIF byte data
	reader := bytes.NewReader(exifData)
	
	// Try to decode EXIF
	x, err := exif.Decode(reader)
	if err != nil {
		log.Printf("[ImageService] Failed to decode EXIF from byte data: %v", err)
		return 1 // Default orientation if EXIF parsing fails
	}

	// Get orientation tag
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		log.Printf("[ImageService] Orientation tag not found in EXIF: %v", err)
		return 1 // Default orientation if tag not found
	}

	orient, err := tag.Int(0)
	if err != nil {
		log.Printf("[ImageService] Failed to parse orientation value: %v", err)
		return 1
	}

	log.Printf("[ImageService] Successfully extracted orientation from EXIF: %d", orient)
	return orient
}

// applyOrientation applies EXIF orientation transformation to image
func (s *ImageService) applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 1:
		// Normal - no transformation needed
		return img
	case 2:
		// Flip horizontal
		return imaging.FlipH(img)
	case 3:
		// Rotate 180°
		return imaging.Rotate180(img)
	case 4:
		// Flip vertical
		return imaging.FlipV(img)
	case 5:
		// Rotate 90° CW + Flip horizontal
		return imaging.FlipH(imaging.Rotate270(img))
	case 6:
		// Rotate 90° CW (most common for iPhone portrait)
		return imaging.Rotate270(img)
	case 7:
		// Rotate 90° CCW + Flip horizontal
		return imaging.FlipH(imaging.Rotate90(img))
	case 8:
		// Rotate 90° CCW
		return imaging.Rotate90(img)
	default:
		// Unknown orientation - return as is
		return img
	}
}

// DeleteImage deletes an image file if it exists
func (s *ImageService) DeleteImage(filePath string) error {
	if filePath == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to delete
	}

	// Delete file
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
