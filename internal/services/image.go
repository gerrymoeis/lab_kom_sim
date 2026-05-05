package services

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
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
// maxDimension: maximum width or height (maintains aspect ratio)
// Returns error if compression fails
func (s *ImageService) CompressAndSave(sourcePath, destPath string, maxDimension int) error {
	// Open source image
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer srcFile.Close()

	// Decode image
	img, _, err := image.Decode(srcFile)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Resize if needed (maintain aspect ratio)
	var resized image.Image
	if width > maxDimension || height > maxDimension {
		resized = imaging.Fit(img, maxDimension, maxDimension, imaging.Lanczos)
	} else {
		resized = img
	}

	// Create destination directory if not exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Encode to JPEG with quality setting
	options := &jpeg.Options{Quality: s.quality}
	if err := jpeg.Encode(destFile, resized, options); err != nil {
		return fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return nil
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
