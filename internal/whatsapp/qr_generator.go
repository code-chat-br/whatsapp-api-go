package whatsapp

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const qrPNGSize = 512

type QRGenerator struct {
	light color.Color
	dark  color.Color
}

func NewQRGenerator(lightHex string, darkHex string) (QRGenerator, error) {
	light, err := parseHexColor(lightHex)
	if err != nil {
		return QRGenerator{}, err
	}
	dark, err := parseHexColor(darkHex)
	if err != nil {
		return QRGenerator{}, err
	}
	return QRGenerator{light: light, dark: dark}, nil
}

func (g QRGenerator) GenerateDataURL(code string) (string, error) {
	qr, err := qrcode.New(code, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("create QR code: %w", err)
	}
	qr.BackgroundColor = g.light
	qr.ForegroundColor = g.dark
	png, err := qr.PNG(qrPNGSize)
	if err != nil {
		return "", fmt.Errorf("render QR png: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

func parseHexColor(value string) (color.RGBA, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return color.RGBA{}, fmt.Errorf("invalid color")
	}
	r, err := strconv.ParseUint(value[0:2], 16, 8)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("invalid red component: %w", err)
	}
	g, err := strconv.ParseUint(value[2:4], 16, 8)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("invalid green component: %w", err)
	}
	b, err := strconv.ParseUint(value[4:6], 16, 8)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("invalid blue component: %w", err)
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xff}, nil
}
