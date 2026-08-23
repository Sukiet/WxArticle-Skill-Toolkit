package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	stddraw "image/draw"
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
	case ".jpg", ".jpeg", ".png", ".gif":
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

	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}

	format = strings.ToLower(format)
	if format == "jpg" {
		format = "jpeg"
	}
	if format == "gif" {
		targetBytes := maxBytes
		if targetCompressedImageSize < maxBytes {
			targetBytes = targetCompressedImageSize
		}

		encoded, err := compressGIF(data, targetBytes)
		if err != nil {
			return 0, 0, err
		}
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			return 0, 0, err
		}
		return originalSize, int64(len(encoded)), nil
	}

	sourceImage, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
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

func compressGIF(data []byte, targetBytes int64) ([]byte, error) {
	sourceGIF, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(sourceGIF.Image) == 0 {
		return nil, errors.New("gif has no frames")
	}

	originalSize := int64(len(data))
	canvasWidth, canvasHeight := gifCanvasSize(sourceGIF)
	renderedFrames := renderGIFFullFrames(sourceGIF)
	scaleFactors := gifScaleFactors(originalSize, targetBytes)
	colorCounts := gifColorCounts(originalSize, targetBytes)

	var lastEncoded []byte
	for _, factor := range scaleFactors {
		targetWidth, targetHeight := scaledDimensions(canvasWidth, canvasHeight, factor)
		preparedFrames := prepareGIFFrames(renderedFrames, targetWidth, targetHeight)

		for _, colorCount := range colorCounts {
			encoded, err := encodeGIFFrames(sourceGIF, preparedFrames, targetWidth, targetHeight, colorCount)
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
	return nil, errors.New("failed to encode gif image")
}

func gifCanvasSize(sourceGIF *gif.GIF) (int, int) {
	width := sourceGIF.Config.Width
	height := sourceGIF.Config.Height
	if width > 0 && height > 0 {
		return width, height
	}
	if len(sourceGIF.Image) == 0 {
		return 1, 1
	}
	return sourceGIF.Image[0].Bounds().Dx(), sourceGIF.Image[0].Bounds().Dy()
}

func gifScaleFactors(originalSize, targetBytes int64) []float64 {
	if originalSize <= targetBytes || targetBytes <= 0 {
		return []float64{1.0}
	}

	ratio := float64(targetBytes) / float64(originalSize)
	estimated := math.Sqrt(ratio)
	if estimated > 1.0 {
		estimated = 1.0
	}
	if estimated < 0.25 {
		estimated = 0.25
	}

	candidates := []float64{
		estimated * 1.10,
		estimated,
		estimated * 0.92,
		estimated * 0.84,
	}

	if ratio > 0.85 {
		candidates = append([]float64{1.0, 0.97}, candidates...)
	}

	return uniqueScaleFactors(candidates)
}

func uniqueScaleFactors(values []float64) []float64 {
	seen := map[int]struct{}{}
	result := make([]float64, 0, len(values))

	for _, value := range values {
		if value > 1.0 {
			value = 1.0
		}
		if value < 0.25 {
			value = 0.25
		}

		rounded := int(math.Round(value * 100))
		if _, exists := seen[rounded]; exists {
			continue
		}
		seen[rounded] = struct{}{}
		result = append(result, float64(rounded)/100.0)
	}

	return result
}

func gifColorCounts(originalSize, targetBytes int64) []int {
	if originalSize <= targetBytes || targetBytes <= 0 {
		return []int{256, 192, 128}
	}

	ratio := float64(targetBytes) / float64(originalSize)
	switch {
	case ratio >= 0.85:
		return []int{256, 224, 192, 160}
	case ratio >= 0.60:
		return []int{224, 192, 160, 128}
	case ratio >= 0.45:
		return []int{192, 160, 128, 96}
	default:
		return []int{160, 128, 96, 64}
	}
}

func prepareGIFFrames(renderedFrames []*image.RGBA, width, height int) []*image.RGBA {
	prepared := make([]*image.RGBA, 0, len(renderedFrames))
	for _, frame := range renderedFrames {
		if frame.Bounds().Dx() == width && frame.Bounds().Dy() == height {
			prepared = append(prepared, frame)
			continue
		}
		prepared = append(prepared, resizeAnimatedFrame(frame, width, height))
	}
	return prepared
}

func encodeGIFFrames(sourceGIF *gif.GIF, preparedFrames []*image.RGBA, targetWidth, targetHeight, colorCount int) ([]byte, error) {
	targetPalette := buildGIFPalette(colorCount)

	result := &gif.GIF{
		Image:           make([]*image.Paletted, 0, len(preparedFrames)),
		Delay:           make([]int, len(preparedFrames)),
		Disposal:        make([]byte, len(preparedFrames)),
		LoopCount:       sourceGIF.LoopCount,
		BackgroundIndex: 0,
		Config: image.Config{
			ColorModel: targetPalette,
			Width:      targetWidth,
			Height:     targetHeight,
		},
	}
	copy(result.Delay, sourceGIF.Delay)
	copy(result.Disposal, sourceGIF.Disposal)

	for _, frame := range preparedFrames {
		paletted := image.NewPaletted(image.Rect(0, 0, targetWidth, targetHeight), targetPalette)
		stddraw.Draw(paletted, paletted.Bounds(), image.NewUniform(color.Transparent), image.Point{}, stddraw.Src)
		stddraw.FloydSteinberg.Draw(paletted, paletted.Bounds(), frame, frame.Bounds().Min)
		result.Image = append(result.Image, paletted)
	}

	buffer := &bytes.Buffer{}
	if err := gif.EncodeAll(buffer, result); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func resizeAnimatedFrame(src image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func renderGIFFullFrames(sourceGIF *gif.GIF) []*image.RGBA {
	if len(sourceGIF.Image) == 0 {
		return nil
	}

	width := sourceGIF.Config.Width
	height := sourceGIF.Config.Height
	if width <= 0 || height <= 0 {
		width = sourceGIF.Image[0].Bounds().Dx()
		height = sourceGIF.Image[0].Bounds().Dy()
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	frames := make([]*image.RGBA, 0, len(sourceGIF.Image))

	for index, frame := range sourceGIF.Image {
		before := cloneRGBA(canvas)
		stddraw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, stddraw.Over)
		frames = append(frames, cloneRGBA(canvas))

		disposal := byte(gif.DisposalNone)
		if index < len(sourceGIF.Disposal) {
			disposal = sourceGIF.Disposal[index]
		}

		switch disposal {
		case gif.DisposalBackground:
			clearRGBA(canvas, frame.Bounds())
		case gif.DisposalPrevious:
			canvas = before
		}
	}

	return frames
}

func buildGIFPalette(colorCount int) color.Palette {
	if colorCount <= 1 {
		return color.Palette{color.Transparent}
	}

	if colorCount >= len(palette.Plan9) {
		return append(color.Palette(nil), palette.Plan9...)
	}

	result := make(color.Palette, 0, colorCount)
	result = append(result, color.Transparent)

	remaining := colorCount - 1
	for index := 0; index < remaining; index++ {
		sourceIndex := index * (len(palette.Plan9) - 1) / remaining
		result = append(result, palette.Plan9[sourceIndex])
	}

	return result
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func clearRGBA(img *image.RGBA, rect image.Rectangle) {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	stddraw.Draw(img, rect, image.NewUniform(color.Transparent), image.Point{}, stddraw.Src)
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
