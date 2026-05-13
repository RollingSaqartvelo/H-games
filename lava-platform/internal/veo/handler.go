package veo

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var safeFilename = regexp.MustCompile(`^[a-z0-9_\-]+$`)

type Handler struct {
	client  *Client
	baseDir string
}

func NewHandler(apiKey, baseDir string) *Handler {
	return &Handler{client: NewClient(apiKey), baseDir: baseDir}
}

type generateReq struct {
	Prompt      string `json:"prompt" binding:"required"`
	Filename    string `json:"filename" binding:"required"`
	Game        string `json:"game"`
	Duration    int    `json:"duration"`
	AspectRatio string `json:"aspect_ratio"`
}

// Generate handles POST /admin/v1/veo/generate
func (h *Handler) Generate(c *gin.Context) {
	var req generateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.generateOne(c, req.Prompt, req.Filename, req.Game, req.Duration, req.AspectRatio)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// bubblePresets are the two video assets needed for the Bubble-Gum game.
var bubblePresets = []struct {
	Filename    string
	Duration    int
	AspectRatio string
	Prompt      string
}{
	{
		Filename: "grow", Duration: 8, AspectRatio: "1:1",
		Prompt: "A magical pink bubblegum bubble self-inflating autonomously, floating in dark deep-purple cosmic candy space. Starts tiny like a marble, grows steadily and smoothly to fill the entire frame. Perfectly round glossy neon-pink surface, bright white specular highlight shifting as it grows, translucent bubble walls glowing hot-pink from within. No people, no hands, no characters, no text. Smooth continuous inflation motion. Neon-pink and violet ambient light. Cinematic premium mobile game animation. Clean dark background.",
	},
	{
		Filename: "pop", Duration: 3, AspectRatio: "1:1",
		Prompt: "A massive glossy neon-pink bubblegum bubble dramatically exploding in slow motion. Micro-cracks form on the surface then catastrophic burst — hot-pink gum shards and sticky translucent strands fly outward radially in all directions, bright shockwave ring of light, vivid neon flash. No people, no hands, no characters, no text. Dark purple-black background. Cinematic slow-motion explosion. Premium mobile game visual effect.",
	},
}

// GenerateBubbleVideos handles POST /admin/v1/veo/preset/bubble
func (h *Handler) GenerateBubbleVideos(c *gin.Context) {
	type result struct {
		Filename string `json:"filename"`
		URL      string `json:"url,omitempty"`
		Error    string `json:"error,omitempty"`
	}
	results := make([]result, len(bubblePresets))
	for i, p := range bubblePresets {
		res, err := h.generateOne(c, p.Prompt, p.Filename, "bubble", p.Duration, p.AspectRatio)
		if err != nil {
			results[i] = result{Filename: p.Filename, Error: err.Error()}
		} else {
			results[i] = result{Filename: p.Filename, URL: res["url"].(string)}
		}
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) generateOne(c *gin.Context, prompt, filename, game string, duration int, aspectRatio string) (map[string]interface{}, error) {
	filename = strings.ToLower(strings.TrimSpace(filename))
	game = strings.ToLower(strings.TrimSpace(game))
	if game == "" {
		game = "bubble"
	}
	if !safeFilename.MatchString(filename) || !safeFilename.MatchString(game) {
		return nil, fmt.Errorf("filename and game must be lowercase alphanumeric/dash/underscore")
	}
	if duration <= 0 {
		duration = 8
	}
	if aspectRatio == "" {
		aspectRatio = "1:1"
	}

	videoBytes, err := h.client.GenerateVideo(c.Request.Context(), prompt, duration, aspectRatio)
	if err != nil {
		return nil, fmt.Errorf("veo: %w", err)
	}

	dir := filepath.Join(h.baseDir, game)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	outPath := filepath.Join(dir, filename+".mp4")
	if err := os.WriteFile(outPath, videoBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return map[string]interface{}{
		"url":  fmt.Sprintf("/assets/%s/%s.mp4", game, filename),
		"path": outPath,
		"size": len(videoBytes),
	}, nil
}
