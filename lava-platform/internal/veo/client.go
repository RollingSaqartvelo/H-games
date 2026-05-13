package veo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	generateURL = "https://generativelanguage.googleapis.com/v1beta/models/veo-2.0-generate-001:predictLongRunning"
	pollBase    = "https://generativelanguage.googleapis.com/v1beta"
)

type Client struct {
	apiKey string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

type generateRequest struct {
	Instances  []instance `json:"instances"`
	Parameters params     `json:"parameters"`
}

type instance struct {
	Prompt string `json:"prompt"`
}

type params struct {
	AspectRatio     string `json:"aspectRatio"`
	DurationSeconds int    `json:"durationSeconds"`
}

type operationResp struct {
	Name     string `json:"name"`
	Done     bool   `json:"done"`
	Response *struct {
		GenerateVideoResponse *struct {
			GeneratedSamples []struct {
				Video struct {
					URI                string `json:"uri"`
					BytesBase64Encoded string `json:"bytesBase64Encoded"`
				} `json:"video"`
			} `json:"generatedSamples"`
		} `json:"generateVideoResponse"`
	} `json:"response"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// GenerateVideo starts a Veo 2 generation and polls until complete, then returns raw MP4 bytes.
func (c *Client) GenerateVideo(ctx context.Context, prompt string, durationSecs int, aspectRatio string) ([]byte, error) {
	body, err := json.Marshal(generateRequest{
		Instances:  []instance{{Prompt: prompt}},
		Parameters: params{AspectRatio: aspectRatio, DurationSeconds: durationSecs},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s?key=%s", generateURL, c.apiKey), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("veo api %d: %s", resp.StatusCode, string(raw))
	}

	var op operationResp
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, fmt.Errorf("parse op: %w", err)
	}

	return c.poll(ctx, op.Name)
}

func (c *Client) poll(ctx context.Context, name string) ([]byte, error) {
	// Strip leading slash if present — pollBase already ends without one
	name = strings.TrimPrefix(name, "/")
	url := fmt.Sprintf("%s/%s?key=%s", pollBase, name, c.apiKey)

	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			resp, err := c.http.Do(req)
			if err != nil {
				return nil, err
			}
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var op operationResp
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("poll parse: %w", err)
			}
			if op.Error != nil {
				return nil, fmt.Errorf("veo error %d: %s", op.Error.Code, op.Error.Message)
			}
			if !op.Done {
				continue
			}

			if op.Response == nil || op.Response.GenerateVideoResponse == nil {
				return nil, fmt.Errorf("veo: empty response")
			}
			samples := op.Response.GenerateVideoResponse.GeneratedSamples
			if len(samples) == 0 {
				return nil, fmt.Errorf("veo: no samples")
			}
			vid := samples[0].Video
			if vid.BytesBase64Encoded != "" {
				return base64.StdEncoding.DecodeString(vid.BytesBase64Encoded)
			}
			if vid.URI != "" {
				return c.download(ctx, vid.URI)
			}
			return nil, fmt.Errorf("veo: no video data in response")
		}
	}
}

func (c *Client) download(ctx context.Context, uri string) ([]byte, error) {
	sep := "?"
	if strings.Contains(uri, "?") {
		sep = "&"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri+sep+"key="+c.apiKey, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
