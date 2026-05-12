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

const apiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-preview-image-generation:generateContent"

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 60 * time.Second},
	}
}

type generateRequest struct {
	Contents         []content        `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	ResponseModalities []string `json:"responseModalities"`
}

type generateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text       string `json:"text,omitempty"`
				InlineData *struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData,omitempty"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// GenerateImage calls Gemini and returns the raw image bytes.
func (c *Client) GenerateImage(ctx context.Context, prompt string) ([]byte, string, error) {
	reqBody := generateRequest{
		Contents: []content{{Parts: []part{{Text: prompt}}}},
		GenerationConfig: generationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("%s?key=%s", apiURL, c.apiKey)
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
		return nil, "", fmt.Errorf("gemini api error %d: %s", resp.StatusCode, string(body))
	}

	var result generateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", err
	}

	for _, cand := range result.Candidates {
		for _, p := range cand.Content.Parts {
			if p.InlineData != nil && len(p.InlineData.Data) > 0 {
				imgBytes, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
				if err != nil {
					return nil, "", fmt.Errorf("base64 decode: %w", err)
				}
				return imgBytes, p.InlineData.MimeType, nil
			}
		}
	}

	return nil, "", fmt.Errorf("gemini returned no image in response")
}
