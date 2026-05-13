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

// ── Outlaw Escape asset generation ───────────────────────────────────────────

// outlawStyle is the exact visual DNA extracted from the reference images.
// Flat geometric low-poly vector illustration — NO gradients, NO outlines.
const outlawStyle = `MANDATORY ART STYLE — match EXACTLY, no deviation:
Flat geometric low-poly vector illustration. Travel poster / WPA mural aesthetic.
All shapes are hard-edged flat-color polygons. ZERO gradients inside any single shape.
No cartoon outlines. Shape edges defined purely by adjacent flat color contrast.
Strong bold simplified geometric silhouettes. Limited tonal range per object.

MANDATORY COLOR PALETTE — exact values, do not substitute:
  Sky clear:       #5A7FA8 / #3A5070 (deep)
  Cloud cream:     #E0CFA8 (highlight), #C8B890 (mid), #8A7888 (shadow), #6A5868 (dark)
  Mountain slate:  #4A5F78, #2A3848 (dark), #8A4A28 (warm), #C4622D (accent russet)
  Ground near:     #C05828 (terracotta), #9A4220 (rust), #D4904A (sand highlight)
  Ground far:      #7A3A18 (dark earth), #4A2810 (deep shadow)
  Scrub/bush:      #3A3A22 (dark olive), #2A3018 (near black-green)
  Tumbleweed:      #8A7848 (straw brown)
  Skull/bone:      #D8C8A0 (bleached)

No photorealism. No texture maps. No soft shadows. Geometric hard-edge only.`

// outlawFloorPanels defines 4 seamlessly-continuable floor tiles.
// These are GROUND STRIP textures — no sky, no horizon, just the desert ground surface.
// Displayed as the bottom 220px of the screen in a side-scrolling game.
// The characters' hooves land at the very top edge of these tiles.
var outlawFloorPanels = []struct {
	Filename string
	Prompt   string
}{
	{
		Filename: "seamless_desert_floor",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Wide horizontal ground tile for a side-scrolling game.
NO SKY. NO CLOUDS. NO HORIZON LINE. NO MOUNTAINS. GROUND ONLY.

This is the desert FLOOR SURFACE viewed from the side, close-up.
The top edge of this image IS the ground surface — the line where earth meets air.
Characters run along this top edge with their feet touching it.

COMPOSITION (top to bottom):
  TOP 15%: Ground surface edge. Hard flat terracotta (#C05828) ground top.
    Small angular rock shard silhouettes and 2-3 low scrub-bush shapes breaking the edge.
  MIDDLE 50%: Packed desert earth. Geometric flat-color strata bands.
    Alternating terracotta (#9A4220), rust-brown (#7A3A18), and orange-sand (#D4904A) layers.
    Angular rock formations embedded in earth. Geometric shapes only.
  BOTTOM 35%: Deep earth shadow. Dark rust (#4A2810) with near-black (#2A1800) base.

DETAILS: 3-4 small angular rock clusters scattered along the top surface.
2-3 dark scrub-bush silhouettes (#3A3A22) at ground level.
Seamless: left and right edges must tile perfectly horizontally.`,
	},
	{
		Filename: "seamless_desert_floor2",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Wide horizontal ground tile for a side-scrolling game.
NO SKY. NO CLOUDS. NO HORIZON LINE. NO MOUNTAINS. GROUND ONLY.
LEFT EDGE MUST MATCH THE PROVIDED REFERENCE IMAGE EXACTLY — continue terrain seamlessly from it.

COMPOSITION (top to bottom):
  TOP 15%: Ground surface edge. Same terracotta level as reference left edge.
    FEATURE: Angular wooden post with bleached skull (#D8C8A0) mounted on top, left-of-center.
    One flat tumbleweed shape (#8A7848) rolling on the ground surface, center area.
    2-3 small angular rock silhouettes at the surface edge.
  MIDDLE 50%: Packed desert earth strata. Same color bands as left reference.
    Terracotta (#9A4220), rust-brown (#7A3A18), sand highlight (#D4904A) geometric layers.
  BOTTOM 35%: Deep dark earth (#4A2810, #2A1800).

Seamless: left edge matches reference exactly; right edge must tile with Panel 3.`,
	},
	{
		Filename: "seamless_desert_floor3",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Wide horizontal ground tile for a side-scrolling game.
NO SKY. NO CLOUDS. NO HORIZON LINE. NO MOUNTAINS. GROUND ONLY.
LEFT EDGE MUST MATCH THE PROVIDED REFERENCE IMAGE EXACTLY — continue terrain seamlessly from it.

COMPOSITION (top to bottom):
  TOP 15%: Ground surface edge. Same terracotta level as reference left edge.
    FEATURE: 2 tall saguaro cactus shapes (#3A3A22, dark olive green) rising FROM the ground surface.
    Simplified geometric cactus: vertical trunk + 2 upward-angled arms. Flat color polygons only.
    2-3 angular rock clusters along the top surface edge.
  MIDDLE 50%: Desert earth strata bands. Slightly warmer tone than Panel 2.
    More orange-sand (#D4904A) layers visible, terracotta and rust beneath.
  BOTTOM 35%: Deep dark earth (#4A2810, #2A1800).

Seamless: left edge matches reference; right edge designed to continue to Panel 4.`,
	},
	{
		Filename: "seamless_desert_floor4",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Wide horizontal ground tile for a side-scrolling game.
NO SKY. NO CLOUDS. NO HORIZON LINE. NO MOUNTAINS. GROUND ONLY.
LEFT EDGE MUST MATCH THE PROVIDED REFERENCE IMAGE EXACTLY — continue terrain seamlessly from it.

COMPOSITION (top to bottom):
  TOP 15%: Ground surface edge. Bright warm desert sand (#D4904A) — sunbaked, hot.
    FEATURES: 3 more saguaro cactus shapes continuing from left edge. Sparse open look.
    Minimal rocks: 1-2 flat angular shapes.
  MIDDLE 50%: Hot sunbaked desert earth. Bright orange-sand (#D4904A) dominant layer.
    Deeper terracotta (#C05828) and rust (#9A4220) strata beneath.
  BOTTOM 35%: Deep earth shadow (#4A2810). Slightly cooler than middle.

Seamless: left edge matches reference; RIGHT EDGE must be designed to loop back to Panel 1
— transition back toward cooler rust-brown terracotta, a few angular rocks reappearing.`,
	},
}

// outlawBGPrompts defines the two sky/background images for the outlaw game.
var outlawBGPrompts = []struct {
	Filename string
	SubDir   string
	Prompt   string
}{
	{
		Filename: "bg_betting_dawn",
		SubDir:   "bg",
		Prompt: outlawStyle + `

FULL SCENE WESTERN LANDSCAPE BACKGROUND — 1920×1080 pixels. 16:9 aspect ratio.
PRE-DAWN SCENE. Before the chase begins. Tense stillness.

SKY: Deep dark blue-grey (#1A2030) upper, transitioning to warm amber-orange at horizon.
A handful of geometric star points in upper corners, fading.
MOUNTAINS: Dense layered silhouettes, near-black to dark-purple, multiple ridges.
Strong atmospheric depth through progressive lightening of distant ridges.
GROUND: Dark earth, barely visible, shadow band at bottom.
MOOD: Ominous, dark, pre-action tension. No characters, no text.
This is the game background seen during the BETTING / waiting phase.`,
	},
	{
		Filename: "bg_running_sunset",
		SubDir:   "bg",
		Prompt: outlawStyle + `

FULL SCENE WESTERN LANDSCAPE BACKGROUND — 1920×1080 pixels. 16:9 aspect ratio.
DRAMATIC GOLDEN HOUR SUNSET. The chase is happening NOW.

SKY: Brilliant orange-amber gradient. Crimson-pink angular cloud formations upper half.
Clouds: geometric faceted shapes, golden highlights, dark shadow bases.
SUN: Not visible directly but intense warm backlight from right side.
MOUNTAINS: Warm russet-orange lit, dark shadow faces, multiple canyon mesa formations.
Strong graphic silhouettes with warm highlights.
GROUND: Rich terracotta, intense golden light, long dark shadow bands.
MOOD: High tension, explosive energy, cinematic drama. No characters, no text.
This is the game background seen DURING the running/chase phase.`,
	},
}

// GenerateOutlawFloors handles POST /admin/v1/gemini/preset/outlaw-floors.
// Generates 4 seamless floor panels with edge-based continuity seams.
func (h *Handler) GenerateOutlawFloors(c *gin.Context) {
	results := make([]batchResult, len(outlawFloorPanels))
	var prevEdge []byte // right-edge crop of previous panel for seam reference

	for i, panel := range outlawFloorPanels {
		var imgBytes []byte
		var err error

		if prevEdge == nil {
			// Panel 1: text-only
			imgBytes, _, err = h.client.GenerateImage(c.Request.Context(), panel.Prompt)
		} else {
			// Panels 2-4: pass previous right edge as inline image reference
			imgBytes, _, err = h.client.GenerateImageWithRef(c.Request.Context(), panel.Prompt, prevEdge, "image/png")
		}
		if err != nil {
			results[i] = batchResult{Filename: panel.Filename, Error: fmt.Sprintf("generate: %v", err)}
			prevEdge = nil
			continue
		}

		// Resize to standard floor tile dimensions (2048×512)
		imgBytes, err = postProcess(imgBytes, false, 0)
		if err != nil {
			results[i] = batchResult{Filename: panel.Filename, Error: fmt.Sprintf("post-process: %v", err)}
			prevEdge = nil
			continue
		}

		// Save to frontend/dist/assets/environment/floor/
		dir := filepath.Join(h.baseDir, "environment", "floor")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			results[i] = batchResult{Filename: panel.Filename, Error: fmt.Sprintf("mkdir: %v", err)}
			prevEdge = nil
			continue
		}
		outPath := filepath.Join(dir, panel.Filename+".png")
		if err := os.WriteFile(outPath, imgBytes, 0o644); err != nil {
			results[i] = batchResult{Filename: panel.Filename, Error: fmt.Sprintf("write: %v", err)}
			prevEdge = nil
			continue
		}

		// Crop right 30% as seam reference for next panel
		edge, cropErr := cropRightEdge(imgBytes, 0.30)
		if cropErr == nil {
			prevEdge = edge
		} else {
			prevEdge = nil
		}

		results[i] = batchResult{
			Filename: panel.Filename,
			URL:      fmt.Sprintf("/assets/environment/floor/%s.png", panel.Filename),
		}
	}

	c.JSON(http.StatusOK, results)
}

// GenerateOutlawBGs handles POST /admin/v1/gemini/preset/outlaw-bg.
func (h *Handler) GenerateOutlawBGs(c *gin.Context) {
	results := make([]batchResult, len(outlawBGPrompts))
	for i, item := range outlawBGPrompts {
		imgBytes, _, err := h.client.GenerateImage(c.Request.Context(), item.Prompt)
		if err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
			continue
		}
		imgBytes, err = postProcess(imgBytes, false, 0)
		if err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
			continue
		}
		dir := filepath.Join(h.baseDir, item.SubDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
			continue
		}
		outPath := filepath.Join(dir, item.Filename+".png")
		if err := os.WriteFile(outPath, imgBytes, 0o644); err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
			continue
		}
		results[i] = batchResult{
			Filename: item.Filename,
			URL:      fmt.Sprintf("/assets/%s/%s.png", item.SubDir, item.Filename),
		}
	}
	c.JSON(http.StatusOK, results)
}

// cropRightEdge extracts the right `fraction` of a PNG as a reference for seam continuity.
func cropRightEdge(data []byte, fraction float64) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	cropX := b.Min.X + int(float64(b.Dx())*(1-fraction))
	cropW := b.Max.X - cropX
	dst := image.NewRGBA(image.Rect(0, 0, cropW, b.Dy()))
	stdraw.Draw(dst, dst.Bounds(), src, image.Point{X: cropX, Y: b.Min.Y}, stdraw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
