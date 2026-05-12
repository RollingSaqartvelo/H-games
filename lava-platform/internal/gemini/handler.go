package gemini

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdraw "image/draw"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
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
	Prompt   string `json:"prompt" binding:"required"`
	Filename string `json:"filename" binding:"required"`
	Game     string `json:"game"`
	RemoveBg bool   `json:"remove_bg"` // strip near-white background → transparent PNG
	Size     int    `json:"size"`      // resize to NxN pixels (0 = no resize)
}

// Generate handles POST /admin/v1/gemini/generate — single image.
func (h *Handler) Generate(c *gin.Context) {
	var req generateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.generateOne(c, req.Prompt, req.Filename, req.Game, req.RemoveBg, req.Size)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

type batchItem struct {
	Prompt   string `json:"prompt" binding:"required"`
	Filename string `json:"filename" binding:"required"`
	Game     string `json:"game"`
	RemoveBg bool   `json:"remove_bg"`
	Size     int    `json:"size"`
}

type batchResult struct {
	Filename string `json:"filename"`
	URL      string `json:"url,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Batch handles POST /admin/v1/gemini/batch — multiple images sequentially.
func (h *Handler) Batch(c *gin.Context) {
	var items []batchItem
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(items) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max 30 items per batch"})
		return
	}
	results := make([]batchResult, len(items))
	for i, item := range items {
		res, err := h.generateOne(c, item.Prompt, item.Filename, item.Game, item.RemoveBg, item.Size)
		if err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
		} else {
			results[i] = batchResult{Filename: item.Filename, URL: res["url"].(string)}
		}
	}
	c.JSON(http.StatusOK, results)
}

// styleBase is appended to every bubble asset prompt for visual consistency.
const styleBase = `Premium dark luxury game art style. Pure white background, completely isolated subject, no shadows on background, clean edges for background removal. High detail, professional illustration, suitable for mobile game.`

// bubbleFramePrompts — all frames needed for Bubble-Gum game, 512x512, white bg.
var bubbleFramePrompts = []batchItem{
	// Background — no bg removal, landscape-ish
	{Filename: "bg", Size: 1024, Game: "bubble", RemoveBg: false,
		Prompt: "Dark luxury candy city skyline at night. Neon pink and magenta glowing buildings. Deep purple-black sky. Candy cane streetlights. Shiny gum road. Cinematic premium game background. No characters, no text, no UI. Wide panoramic view."},

	// Character
	{Filename: "char_idle", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish confident teenager in dark luxury streetwear, standing upright, mouth closed chewing gum, no bubble yet, full body portrait, centered on pure white background, premium cartoon game art. " + styleBase},

	// Bubble growth stages
	{Filename: "stage_01", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager blowing a tiny grape-sized glossy pink bubblegum bubble, full body portrait, bubble clearly visible, pure white background. " + styleBase},
	{Filename: "stage_02", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager blowing a small tennis-ball-sized glossy magenta bubblegum bubble, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_03", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager blowing a softball-sized glossy neon-pink bubblegum bubble with specular highlight, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_04", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager blowing a volleyball-sized glossy hot-pink bubblegum bubble, gum slightly stretched, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_05", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager blowing a basketball-sized glossy neon-pink bubblegum bubble expanding upward-right, gum visibly thin and stretched, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_06", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager struggling with a huge glossy translucent pink bubblegum bubble bigger than their head, expanding diagonally upper-right, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_07", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager with an enormous double-head-sized glossy translucent pink bubblegum bubble, extreme diagonal expansion upper-right, surface tension visible, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_08", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager barely controlling a colossal triple-head-sized pink bubblegum bubble with stress lines on surface, rainbow refraction, extreme diagonal upper-right, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_09", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager with a massive unstable pink bubblegum bubble four times head size, veins of stress, glowing from within, about to pop, diagonal upper-right at maximum, full body portrait, pure white background. " + styleBase},
	{Filename: "stage_10", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager with a colossal critical pink bubblegum bubble filling most of frame, micro-cracks forming, extreme neon glow, maximum tension, pure white background. " + styleBase},

	// Wobble frames
	{Filename: "wobble_a", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A huge neon-pink glossy bubblegum bubble deformed slightly to the left, wobbling, extreme tension, gloss highlights shifting left, isolated on pure white background. " + styleBase},
	{Filename: "wobble_b", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A huge neon-pink glossy bubblegum bubble deformed slightly to the right, wobbling, extreme tension, gloss highlights shifting right, isolated on pure white background. " + styleBase},

	// Crash sequence
	{Filename: "pop_crack", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A huge pink bubblegum bubble with a visible micro-crack splitting open, pink glow escaping, surface rupturing, dramatic tension, isolated on pure white background. " + styleBase},
	{Filename: "pop_burst", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "Explosive pink bubblegum bubble POP, neon-pink gum fragments flying outward in all directions, shockwave ring, dramatic burst explosion, isolated on pure white background. " + styleBase},
	{Filename: "pop_splat", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A stylish teenager completely covered in sticky pink bubblegum after a bubble explosion, shocked expression, pink gum splattered on face and clothes, pure white background. " + styleBase},
}

// GenerateBubbleFrames handles POST /admin/v1/gemini/preset/bubble
func (h *Handler) GenerateBubbleFrames(c *gin.Context) {
	results := make([]batchResult, len(bubbleFramePrompts))
	for i, item := range bubbleFramePrompts {
		res, err := h.generateOne(c, item.Prompt, item.Filename, item.Game, item.RemoveBg, item.Size)
		if err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
		} else {
			results[i] = batchResult{Filename: item.Filename, URL: res["url"].(string)}
		}
	}
	c.JSON(http.StatusOK, results)
}

// generateOne is the shared image-generation + post-processing + save logic.
func (h *Handler) generateOne(c *gin.Context, prompt, filename, game string, removeBg bool, size int) (map[string]interface{}, error) {
	filename = strings.ToLower(strings.TrimSpace(filename))
	game = strings.ToLower(strings.TrimSpace(game))
	if game == "" {
		game = "bubble"
	}
	if !safeFilename.MatchString(filename) || !safeFilename.MatchString(game) {
		return nil, fmt.Errorf("filename and game must be lowercase alphanumeric/dash/underscore")
	}

	imgBytes, _, err := h.client.GenerateImage(c.Request.Context(), prompt)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}

	// Post-process: remove white background + resize
	imgBytes, err = postProcess(imgBytes, removeBg, size)
	if err != nil {
		return nil, fmt.Errorf("post-process: %w", err)
	}

	dir := filepath.Join(h.baseDir, game)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	outPath := filepath.Join(dir, filename+".png")
	if err := os.WriteFile(outPath, imgBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return map[string]interface{}{
		"url":  fmt.Sprintf("/assets/%s/%s.png", game, filename),
		"path": outPath,
		"size": len(imgBytes),
	}, nil
}

// postProcess removes near-white background (→ transparent RGBA) and resizes.
func postProcess(data []byte, removeBg bool, targetSize int) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Resize if needed
	if targetSize > 0 && (w != targetSize || h != targetSize) {
		dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
		draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
		src = dst
		bounds = src.Bounds()
	}

	if !removeBg {
		var buf bytes.Buffer
		if err := png.Encode(&buf, src); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// Convert to RGBA and remove near-white pixels
	rgba := image.NewRGBA(bounds)
	stdraw.Draw(rgba, bounds, src, bounds.Min, stdraw.Src)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := rgba.RGBAAt(x, y)
			if isNearWhite(c) {
				rgba.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// isNearWhite returns true for pixels that are very close to white.
func isNearWhite(c color.RGBA) bool {
	const threshold = 230
	return c.R > threshold && c.G > threshold && c.B > threshold
}
