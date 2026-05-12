package gemini

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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
	Prompt   string `json:"prompt" binding:"required"`
	Filename string `json:"filename" binding:"required"`
	Game     string `json:"game"`
}

// Generate handles POST /admin/v1/gemini/generate — single image.
func (h *Handler) Generate(c *gin.Context) {
	var req generateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.generateOne(c, req.Prompt, req.Filename, req.Game)
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
}

type batchResult struct {
	Filename string `json:"filename"`
	URL      string `json:"url,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Batch handles POST /admin/v1/gemini/batch — generate multiple images sequentially.
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
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Sequential to avoid rate limits
	for i, item := range items {
		wg.Add(1)
		go func(idx int, it batchItem) {
			defer wg.Done()
			res, err := h.generateOne(c, it.Prompt, it.Filename, it.Game)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[idx] = batchResult{Filename: it.Filename, Error: err.Error()}
			} else {
				results[idx] = batchResult{Filename: it.Filename, URL: res["url"].(string)}
			}
		}(i, item)
	}
	wg.Wait()
	c.JSON(http.StatusOK, results)
}

// bubbleFramePrompts defines all frames needed for Bubble-Gum game.
// Style is locked so every frame is visually consistent.
var bubbleFramePrompts = []batchItem{
	// Background
	{Filename: "bg", Prompt: "A dark luxury candy city skyline at night, neon pink and magenta glow, candy buildings, glowing gum streetlights, deep purple sky, premium game background, no characters, no text, wide cinematic", Game: "bubble"},
	// Character + bubble stages
	{Filename: "char_idle", Prompt: "A stylish young person standing confidently, dark luxury outfit, chewing gum but no bubble yet, transparent PNG, premium game art, dark background, centered, full body portrait", Game: "bubble"},
	{Filename: "stage_01", Prompt: "A stylish young person blowing a tiny pink bubble gum bubble, bubble is very small (grape-sized), gloss pink bubble, dark luxury style, premium game art, transparent PNG, centered portrait", Game: "bubble"},
	{Filename: "stage_02", Prompt: "A stylish young person blowing a small pink bubble gum bubble (tennis ball size), glossy magenta bubble, shiny highlights, premium dark luxury game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_03", Prompt: "A stylish young person blowing a medium pink bubble gum bubble (softball size), glossy neon pink bubble with white highlight specular, dark background, premium game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_04", Prompt: "A stylish young person blowing a medium-large pink bubble (volleyball size), glossy magenta bubble, slight gum stretch visible at lips, premium dark luxury game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_05", Prompt: "A stylish young person blowing a large pink bubble (basketball size), glossy hot-pink bubble, gum visibly stretching thin, slight upward diagonal expansion, premium dark luxury game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_06", Prompt: "A stylish young person blowing a very large pink bubble gum bubble (bigger than head), glossy neon-pink with slight translucency, gum stretched thin, upward-right diagonal expansion, premium game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_07", Prompt: "A stylish young person struggling to blow an enormous pink bubble gum bubble (twice head size), ultra-glossy, translucent edges showing inner glow, expanding diagonally up-right, premium dark luxury game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_08", Prompt: "A stylish young person with a gigantic pink bubble gum bubble (3x head size), extreme gloss, veins of stress visible in gum surface, diagonal up-right movement, wobbling, premium game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_09", Prompt: "A stylish young person barely holding a colossal semi-transparent pink bubble (4x head size), surface cracking stress lines, rainbow refraction in gum, extreme tension, diagonal up-right, premium dark luxury game art, transparent PNG", Game: "bubble"},
	{Filename: "stage_10", Prompt: "A stylish young person with a massive unstable bubble gum bubble filling most of frame, pink and magenta, glowing from within, extreme surface tension lines, about to pop, diagonal up-right expansion maximum, premium dark luxury game art, transparent PNG", Game: "bubble"},
	// Wobble frames
	{Filename: "wobble_a", Prompt: "A huge neon-pink bubble gum bubble wobbling left, slightly deformed left, gloss highlights shifting, extreme tension, dark luxury game art, transparent PNG", Game: "bubble"},
	{Filename: "wobble_b", Prompt: "A huge neon-pink bubble gum bubble wobbling right, slightly deformed right, gloss highlights shifting, extreme tension, dark luxury game art, transparent PNG", Game: "bubble"},
	// Crash / pop frames
	{Filename: "pop_crack", Prompt: "A huge pink bubble gum bubble showing a micro-crack at the surface, gum splitting, tension at breaking point, pink glow escaping through crack, premium dark luxury game art, transparent PNG", Game: "bubble"},
	{Filename: "pop_burst", Prompt: "A pink bubble gum bubble EXPLODING outward, burst of neon-pink gum fragments flying in all directions, shockwave ring, dramatic explosion, premium dark luxury crash moment game art, transparent PNG", Game: "bubble"},
	{Filename: "pop_splat", Prompt: "Pink gum splattered everywhere after bubble pop, sticky gum fragments on screen edges, person covered in pink gum, shocked expression, premium dark luxury game art, transparent PNG", Game: "bubble"},
}

// GenerateBubbleFrames handles POST /admin/v1/gemini/preset/bubble
// Generates all game frames for Bubble-Gum in sequence.
func (h *Handler) GenerateBubbleFrames(c *gin.Context) {
	results := make([]batchResult, len(bubbleFramePrompts))
	for i, item := range bubbleFramePrompts {
		res, err := h.generateOne(c, item.Prompt, item.Filename, item.Game)
		if err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
		} else {
			results[i] = batchResult{Filename: item.Filename, URL: res["url"].(string)}
		}
	}
	c.JSON(http.StatusOK, results)
}

// generateOne is the shared image-generation + save logic.
func (h *Handler) generateOne(c *gin.Context, prompt, filename, game string) (map[string]interface{}, error) {
	filename = strings.ToLower(strings.TrimSpace(filename))
	game = strings.ToLower(strings.TrimSpace(game))
	if game == "" {
		game = "bubble"
	}
	if !safeFilename.MatchString(filename) || !safeFilename.MatchString(game) {
		return nil, fmt.Errorf("filename and game must be lowercase alphanumeric/dash/underscore")
	}

	imgBytes, mimeType, err := h.client.GenerateImage(c.Request.Context(), prompt)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}

	ext := ".png"
	if strings.Contains(mimeType, "jpeg") {
		ext = ".jpg"
	}

	dir := filepath.Join(h.baseDir, game)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	outPath := filepath.Join(dir, filename+ext)
	if err := os.WriteFile(outPath, imgBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return map[string]interface{}{
		"url":  fmt.Sprintf("/assets/%s/%s%s", game, filename, ext),
		"path": outPath,
		"size": len(imgBytes),
		"mime": mimeType,
	}, nil
}
