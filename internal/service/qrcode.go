package service

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// GenerateQRCode returns PNG bytes for a QR code pointing to the given URL.
func GenerateQRCode(url string) ([]byte, error) {
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("generating QR code: %w", err)
	}
	return png, nil
}
