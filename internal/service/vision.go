package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// CVResult contains the result of a computer vision analysis.
type CVResult struct {
	Tiles  []string // detected tiles, e.g.: ["6-4", "2-1", "0-0"]
	Points int      // sum of points
}

type roboflowResponse struct {
	Predictions []struct {
		Class      string  `json:"class"`
		Confidence float64 `json:"confidence"`
	} `json:"predictions"`
}

// AnalyzeImage analyzes an image and returns the detected tiles and point total.
// Returns nil, -1, nil if ROBOFLOW_API_KEY is not set (manual entry required).
func AnalyzeImage(imageBytes []byte) (*CVResult, error) {
	apiKey := os.Getenv("ROBOFLOW_API_KEY")
	if apiKey == "" {
		return &CVResult{Points: -1}, nil
	}

	model := os.Getenv("ROBOFLOW_MODEL")
	if model == "" {
		model = "domino-detection"
	}
	version := os.Getenv("ROBOFLOW_VERSION")
	if version == "" {
		version = "1"
	}

	encoded := base64.StdEncoding.EncodeToString(imageBytes)
	payload := map[string]string{"image": encoded}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload: %w", err)
	}

	url := fmt.Sprintf("https://detect.roboflow.com/%s/%s?api_key=%s", model, version, apiKey)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("calling Roboflow API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("roboflow API returned status %d", resp.StatusCode)
	}

	var rfResp roboflowResponse
	if err := json.NewDecoder(resp.Body).Decode(&rfResp); err != nil {
		return nil, fmt.Errorf("decoding Roboflow response: %w", err)
	}

	result := &CVResult{}
	for _, pred := range rfResp.Predictions {
		pts, err := parseDominoClass(pred.Class)
		if err != nil {
			continue
		}
		result.Points += pts
		result.Tiles = append(result.Tiles, normalizeTile(pred.Class))
	}

	return result, nil
}

// ApplySpecialRules applies Pontinho special rules after confirmation:
//   - Blank tile alone (only "0-0" in hand) → worth 12 points.
//   - Returns the final points to use.
func ApplySpecialRules(tiles []string, points int) int {
	if len(tiles) == 1 && tiles[0] == "0-0" {
		return 12
	}
	return points
}

// CountDotsFromImage maintains compatibility — returns only the total points.
func CountDotsFromImage(imageBytes []byte) (int, error) {
	r, err := AnalyzeImage(imageBytes)
	if err != nil {
		return 0, err
	}
	return r.Points, nil
}

// parseDominoClass parseia "6-4" e retorna 10.
func parseDominoClass(class string) (int, error) {
	parts := strings.SplitN(class, "-", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid domino class: %s", class)
	}
	a, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, fmt.Errorf("invalid part A: %s", parts[0])
	}
	b, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, fmt.Errorf("invalid part B: %s", parts[1])
	}
	return a + b, nil
}

// normalizeTile normalizes tile notation to "min-max" (e.g.: "4-6" → "4-6", "6-4" → "4-6").
// This ensures that "6-4" and "4-6" are treated as the same tile.
func normalizeTile(class string) string {
	parts := strings.SplitN(class, "-", 2)
	if len(parts) != 2 {
		return class
	}
	a, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return class
	}
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("%d-%d", a, b)
}
