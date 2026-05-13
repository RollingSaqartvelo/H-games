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
const styleBase = `NO people, NO humans, NO characters, NO hands, NO faces. Pure white background, completely isolated subject, no shadows on background, clean crisp edges for background removal. Premium candy game art style, vibrant neon colors, high detail, professional illustration, suitable for mobile game.`

// bubbleFramePrompts — all frames needed for Bubble-Gum game, 512x512, white bg.
// The bubble inflates by itself — no characters, no people at all.
var bubbleFramePrompts = []batchItem{
	// Background — dark candy world, no bg removal
	{Filename: "bg", Size: 1024, Game: "bubble", RemoveBg: false,
		Prompt: "Dark luxury candy dreamworld at night. Neon pink and magenta glowing candy mountains. Deep purple-black sky with sugar-crystal stars. Giant lollipop trees. Shiny gum river. Cinematic premium mobile game background. No people, no characters, no text, no UI. Wide panoramic view."},

	// Idle state — a small flat piece of gum before inflation
	{Filename: "char_idle", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A single small round flat piece of shiny pink bubblegum candy, glossy like a jewel, soft neon-pink color with a subtle highlight, cute minimal candy style, centered perfectly, isolated on pure white background. No people, no hands. " + styleBase},

	// Bubble growth stages — the bubble inflates by itself, floating in air
	{Filename: "stage_01", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A tiny grape-sized self-inflating pink bubblegum bubble floating in air, glossy surface, soft specular highlight, magical glow, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_02", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A small tennis-ball-sized self-inflating glossy magenta bubblegum bubble floating in air, beautiful specular highlight, magical inner glow, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_03", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A softball-sized self-inflating glossy neon-pink bubblegum bubble floating in air, strong specular highlight, subtle inner luminance, translucent surface, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_04", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A volleyball-sized self-inflating glossy hot-pink bubblegum bubble floating in air, gum surface slightly stretched, vivid inner glow, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_05", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A basketball-sized self-inflating glossy neon-pink bubblegum bubble floating in air, surface visibly thinning, bright inner light, expanding upward, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_06", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A large head-sized self-inflating glossy translucent pink bubblegum bubble floating in air, glowing neon-pink from within, surface tension beginning to show, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_07", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A huge double-head-sized self-inflating glossy translucent pink bubblegum bubble floating in air, bright neon core glow, surface tension veins forming, rainbow refraction at edges, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_08", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A colossal triple-sized self-inflating glossy translucent pink bubblegum bubble floating in air, stress lines on surface, intense rainbow refraction, white-hot core glow, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_09", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A massive self-inflating pink bubblegum bubble at critical size, deep stress cracks forming on surface, blinding neon-pink inner glow, veins of tension spreading, about to burst, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "stage_10", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A colossal self-inflating pink bubblegum bubble filling almost the entire frame, micro-fractures spreading across surface, extreme neon glow, maximum tension, ghostly translucent walls, no people, no hands, isolated on pure white background. " + styleBase},

	// Wobble frames — bubble sways side to side
	{Filename: "wobble_a", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A huge neon-pink glossy bubblegum bubble gently deformed squashed to the left, wobbling mid-air, gloss highlight shifted left, extreme surface tension, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "wobble_b", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A huge neon-pink glossy bubblegum bubble gently deformed squashed to the right, wobbling mid-air, gloss highlight shifted right, extreme surface tension, no people, no hands, isolated on pure white background. " + styleBase},

	// Crash sequence — the bubble pops on its own
	{Filename: "pop_crack", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "A massive pink bubblegum bubble with a dramatic crack splitting open, hot-pink glow escaping through the rupture, surface tearing apart, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "pop_burst", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "Explosive pink bubblegum bubble POP mid-air, neon-pink gum shards and sticky strands flying outward in all directions, bright shockwave ring, dramatic burst explosion, no people, no hands, isolated on pure white background. " + styleBase},
	{Filename: "pop_splat", Size: 512, Game: "bubble", RemoveBg: true,
		Prompt: "Pink bubblegum explosion aftermath — scattered sticky pink gum blobs and strands splattered in all directions, dripping gum droplets, candy fragments, no people, no hands, isolated on pure white background. " + styleBase},
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
