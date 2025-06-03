package thumbnail

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"time"

	"github.com/pdxmph/imgupv2/pkg/duplicate"

	// Import image format handlers
	_ "image/gif"
)

// Generator handles thumbnail generation and caching
type Generator struct {
	cache *duplicate.SQLiteCache
}

// NewGenerator creates a new thumbnail generator
func NewGenerator(cache *duplicate.SQLiteCache) *Generator {
	return &Generator{cache: cache}
}

// GetCachedThumbnail retrieves a thumbnail from cache by key
func (g *Generator) GetCachedThumbnail(ctx context.Context, key string) (*duplicate.Thumbnail, error) {
	if g.cache == nil {
		return nil, fmt.Errorf("no cache available")
	}
	return g.cache.GetThumbnail(ctx, key)
}

// SaveThumbnail saves a thumbnail to cache
func (g *Generator) SaveThumbnail(thumb *duplicate.Thumbnail) error {
	if g.cache == nil {
		return fmt.Errorf("no cache available")
	}
	return g.cache.SaveThumbnail(thumb)
}

// ImageInfo contains information about an image
type ImageInfo struct {
	Width    int
	Height   int
	FileSize int64
	MD5Hash  string
}

// Result contains the thumbnail and image info
type Result struct {
	ThumbnailData string // base64 encoded
	Info          ImageInfo
}

// Generate creates or retrieves a thumbnail for the given image path
func (g *Generator) Generate(ctx context.Context, imagePath string, maxSize int) (*Result, error) {
	// Get file info and hash
	info, err := g.getImageInfo(imagePath)
	if err != nil {
		return nil, fmt.Errorf("get image info: %w", err)
	}

	// Check cache first
	if g.cache != nil {
		thumb, err := g.cache.GetThumbnail(ctx, info.MD5Hash)
		if err == nil && thumb != nil {
			return &Result{
				ThumbnailData: thumb.ThumbnailData,
				Info:          *info,
			}, nil
		}
	}

	// Generate thumbnail
	thumbData, err := g.generateThumbnail(imagePath, maxSize)
	if err != nil {
		return nil, fmt.Errorf("generate thumbnail: %w", err)
	}

	// Save to cache
	if g.cache != nil {
		thumb := &duplicate.Thumbnail{
			FileMD5:       info.MD5Hash,
			ThumbnailData: thumbData,
			Width:         info.Width,
			Height:        info.Height,
			FileSize:      info.FileSize,
			CreatedAt:     time.Now(),
		}
		_ = g.cache.SaveThumbnail(thumb) // Ignore cache errors
	}

	return &Result{
		ThumbnailData: thumbData,
		Info:          *info,
	}, nil
}

// getImageInfo extracts image metadata and calculates MD5
func (g *Generator) getImageInfo(imagePath string) (*ImageInfo, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Calculate MD5 while reading
	hasher := md5.New()
	reader := io.TeeReader(file, hasher)

	// Decode image to get dimensions
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Read the rest of the file for complete hash
	_, _ = io.Copy(hasher, file)

	bounds := img.Bounds()
	return &ImageInfo{
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		FileSize: stat.Size(),
		MD5Hash:  fmt.Sprintf("%x", hasher.Sum(nil)),
	}, nil
}

// generateThumbnail creates a thumbnail from an image file
func (g *Generator) generateThumbnail(imagePath string, maxSize int) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Decode the image
	img, format, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	// Calculate new dimensions maintaining aspect ratio
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	var newWidth, newHeight int
	if width > height {
		newWidth = maxSize
		newHeight = int(float64(height) * float64(maxSize) / float64(width))
	} else {
		newHeight = maxSize
		newWidth = int(float64(width) * float64(maxSize) / float64(height))
	}

	// Create thumbnail using simple nearest-neighbor for speed
	thumb := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	
	// Simple scaling - for better quality we'd use a proper resampling algorithm
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := x * width / newWidth
			srcY := y * height / newHeight
			thumb.Set(x, y, img.At(srcX, srcY))
		}
	}

	// Encode to JPEG for smaller size
	var buf bytes.Buffer
	if format == "png" && g.hasTransparency(img) {
		// Keep PNG format if there's transparency
		err = png.Encode(&buf, thumb)
	} else {
		// Use JPEG for photos
		err = jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80})
	}
	
	if err != nil {
		return "", fmt.Errorf("encode thumbnail: %w", err)
	}

	// Convert to base64
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// hasTransparency checks if an image has any transparent pixels
func (g *Generator) hasTransparency(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xffff {
				return true
			}
		}
	}
	return false
}

// GenerateWithoutCache creates a thumbnail without caching (for temporary files)
func GenerateThumbnail(imagePath string, maxSize int) (string, error) {
	g := &Generator{cache: nil}
	thumbData, err := g.generateThumbnail(imagePath, maxSize)
	if err != nil {
		return "", err
	}
	return thumbData, nil
}
