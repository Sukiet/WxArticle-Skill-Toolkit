package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const maxUploadImageSize = 10 * 1024 * 1024
const targetCompressedImageSize = int64(float64(maxUploadImageSize) * 0.95)

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func isSupportedCompressImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func compressImageToLimit(path string, maxBytes int64) (int64, int64, error) {
	originalSize, err := fileSize(path)
	if err != nil {
		return 0, 0, err
	}
	if originalSize <= maxBytes {
		return originalSize, originalSize, nil
	}
	if !isSupportedCompressImage(path) {
		return 0, 0, fmt.Errorf("unsupported image format for compress: %s", filepath.Ext(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}

	sourceImage, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}

	format = strings.ToLower(format)
	if format == "jpg" {
		format = "jpeg"
	}
	if format != "jpeg" && format != "png" {
		return 0, 0, fmt.Errorf("unsupported decoded image format for compress: %s", format)
	}

	targetBytes := maxBytes
	if targetCompressedImageSize < maxBytes {
		targetBytes = targetCompressedImageSize
	}

	encoded, err := compressImageBytes(sourceImage, format, targetBytes)
	if err != nil {
		return 0, 0, err
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return 0, 0, err
	}
	return originalSize, int64(len(encoded)), nil
}

func compressImageBytes(sourceImage image.Image, format string, targetBytes int64) ([]byte, error) {
	switch format {
	case "jpeg":
		return compressJPEG(sourceImage, targetBytes)
	case "png":
		return compressPNG(sourceImage, targetBytes)
	default:
		return nil, errors.New("unsupported image format")
	}
}

func compressJPEG(sourceImage image.Image, targetBytes int64) ([]byte, error) {
	qualities := []int{92, 88, 84, 80}
	scaleFactors := []float64{1.0, 0.95, 0.90, 0.85}

	var lastEncoded []byte
	for _, factor := range scaleFactors {
		current := sourceImage
		if factor < 1.0 {
			width, height := scaledDimensions(sourceImage.Bounds().Dx(), sourceImage.Bounds().Dy(), factor)
			current = resizeImage(sourceImage, width, height)
		}

		for _, quality := range qualities {
			encoded, err := encodeJPEG(current, quality)
			if err != nil {
				return nil, err
			}
			lastEncoded = encoded
			if int64(len(encoded)) <= targetBytes {
				return encoded, nil
			}
		}
	}

	if len(lastEncoded) > 0 {
		return nil, fmt.Errorf("image is still larger than %d bytes after compression", targetBytes)
	}
	return nil, errors.New("failed to encode jpeg image")
}

func compressPNG(sourceImage image.Image, targetBytes int64) ([]byte, error) {
	scaleFactors := []float64{1.0, 0.95, 0.90, 0.85}

	var lastEncoded []byte
	for _, factor := range scaleFactors {
		current := sourceImage
		if factor < 1.0 {
			width, height := scaledDimensions(sourceImage.Bounds().Dx(), sourceImage.Bounds().Dy(), factor)
			current = resizeImage(sourceImage, width, height)
		}

		encoded, err := encodePNG(current)
		if err != nil {
			return nil, err
		}
		lastEncoded = encoded
		if int64(len(encoded)) <= targetBytes {
			return encoded, nil
		}
	}

	if len(lastEncoded) > 0 {
		return nil, fmt.Errorf("image is still larger than %d bytes after compression", targetBytes)
	}
	return nil, errors.New("failed to encode png image")
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if err := jpeg.Encode(buffer, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodePNG(img image.Image) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func scaledDimensions(width, height int, factor float64) (int, int) {
	if width <= 1 || height <= 1 {
		return width, height
	}
	nextWidth := int(math.Round(float64(width) * factor))
	nextHeight := int(math.Round(float64(height) * factor))
	if nextWidth < 1 {
		nextWidth = 1
	}
	if nextHeight < 1 {
		nextHeight = 1
	}
	return nextWidth, nextHeight
}

func init() {
	image.RegisterFormat("jpeg", "jpeg", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("jpg", "jpg", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
	image.RegisterFormat("gif", "gif", gif.Decode, gif.DecodeConfig)
}
