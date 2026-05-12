package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Imagen 4 — best available image generation model on this key
const imagenURL = "https://generativelanguage.googleapis.com/v1beta/models/imagen-4.0-generate-001:predict"

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 120 * time.Second},
	}
}

type imagenRequest struct {
	Instances  []imagenInstance  `json:"instances"`
	Parameters imagenParameters  `json:"parameters"`
}

type imagenInstance struct {
	Prompt string `json:"prompt"`
}

type imagenParameters struct {
	SampleCount int    `json:"sampleCount"`
	AspectRatio string `json:"aspectRatio"`
}

type imagenResponse struct {
	Predictions []struct {
		BytesBase64Encoded string `json:"bytesBase64Encoded"`
		MimeType           string `json:"mimeType"`
	} `json:"predictions"`
}

// GenerateImage calls Imagen 4 and returns raw image bytes + mime type.
func (c *Client) GenerateImage(ctx context.Context, prompt string) ([]byte, string, error) {
	reqBody := imagenRequest{
		Instances:  []imagenInstance{{Prompt: prompt}},
		Parameters: imagenParameters{SampleCount: 1, AspectRatio: "1:1"},
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("%s?key=%s", imagenURL, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("imagen api error %d: %s", resp.StatusCode, string(body))
	}

	var result imagenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", err
	}

	if len(result.Predictions) == 0 || result.Predictions[0].BytesBase64Encoded == "" {
		return nil, "", fmt.Errorf("imagen returned no image")
	}

	pred := result.Predictions[0]
	imgBytes, err := base64.StdEncoding.DecodeString(pred.BytesBase64Encoded)
	if err != nil {
		return nil, "", fmt.Errorf("base64 decode: %w", err)
	}

	mimeType := pred.MimeType
	if mimeType == "" {
		mimeType = "image/png"
	}

	return imgBytes, mimeType, nil
}
