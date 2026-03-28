package service

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
)

const (
	MaxImageDimension = 1200 // px
	JPEGQuality       = 72   // 0-100
)

// CompressImage receives image bytes (JPEG or PNG), resizes if larger than
// MaxImageDimension in any dimension and re-encodes as JPEG with reduced quality.
// Returns the compressed bytes and the extension ".jpg".
func CompressImage(data []byte) ([]byte, error) {
	mime, err := detectImageMIME(data)
	if err != nil {
		return nil, err
	}

	var src image.Image
	switch mime {
	case "image/jpeg":
		src, err = jpeg.Decode(bytes.NewReader(data))
	case "image/png":
		src, err = png.Decode(bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("unsupported format: %s", mime)
	}
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	resized := downsample(src, MaxImageDimension, MaxImageDimension)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, fmt.Errorf("encoding JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// ValidateImageMIME checks the magic bytes to determine if the file is a supported image.
func ValidateImageMIME(data []byte) error {
	_, err := detectImageMIME(data)
	return err
}

func detectImageMIME(data []byte) (string, error) {
	if len(data) < 4 {
		return "", fmt.Errorf("file too small to be an image")
	}
	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", nil
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png", nil
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		strings.EqualFold(string(data[8:12]), "WEBP") {
		return "image/webp", nil // accepted but converted via fallback
	}
	return "", fmt.Errorf("unsupported file type: please upload a JPEG or PNG image")
}

// downsample resizes an image to fit in maxW x maxH preserving aspect ratio.
// Uses nearest-neighbor — sufficient for domino photos for CV.
func downsample(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w, h := b.Max.X-b.Min.X, b.Max.Y-b.Min.Y

	if w <= maxW && h <= maxH {
		return src
	}

	scaleW := float64(maxW) / float64(w)
	scaleH := float64(maxH) / float64(h)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	invScale := 1.0 / scale
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := b.Min.X + int(float64(x)*invScale)
			srcY := b.Min.Y + int(float64(y)*invScale)
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}
