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

const generateURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-image:generateContent"

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

type generateRequest struct {
	Contents         []gcontent       `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type gcontent struct {
	Parts []gpart `json:"parts"`
}

type gpart struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inlineData,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
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

// GenerateImage calls Gemini with a text-only prompt.
func (c *Client) GenerateImage(ctx context.Context, prompt string) ([]byte, string, error) {
	return c.generateWithParts(ctx, []gpart{{Text: prompt}})
}

// GenerateImageWithRef calls Gemini with an inline image reference (e.g. seam edge) plus a text prompt.
func (c *Client) GenerateImageWithRef(ctx context.Context, prompt string, refImg []byte, refMime string) ([]byte, string, error) {
	parts := []gpart{
		{InlineData: &inlineData{MimeType: refMime, Data: base64.StdEncoding.EncodeToString(refImg)}},
		{Text: prompt},
	}
	return c.generateWithParts(ctx, parts)
}

func (c *Client) generateWithParts(ctx context.Context, parts []gpart) ([]byte, string, error) {
	reqBody := generateRequest{
		Contents:         []gcontent{{Parts: parts}},
		GenerationConfig: generationConfig{ResponseModalities: []string{"IMAGE"}},
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("%s?key=%s", generateURL, c.apiKey)
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
			if p.InlineData != nil && p.InlineData.Data != "" {
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
