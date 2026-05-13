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
// Flat geometric painted style — bold flat color shapes like a WPA mural.
const outlawStyle = `MANDATORY ART STYLE — match EXACTLY, no deviation:
PAINTED FLAT-COLOR STYLE. Like a WPA New Deal mural or national park travel poster painted with gouache.
Everything looks hand-painted with broad flat brush strokes forming geometric shapes.
NO photorealism. NO 3D rendering. NO gradients inside any shape.
Each shape = one solid flat paint color. Hard sharp edges between adjacent colors.
Looks like illustration art painted on a flat surface — bold, graphic, mural-style.
Angular geometric simplified shapes for everything: clouds, mountains, earth, plants.
Rich deep saturated colors typical of 1930s-40s poster art.

MANDATORY COLOR PALETTE — exact values:
  Sky clear:       #5A7FA8 / #3A5070 (deep)
  Cloud cream:     #E0CFA8 (highlight), #C8B890 (mid), #8A7888 (shadow), #6A5868 (dark)
  Mountain slate:  #4A5F78, #2A3848 (dark), #8A4A28 (warm), #C4622D (accent russet)
  Ground near:     #C05828 (terracotta), #9A4220 (rust), #D4904A (sand highlight)
  Ground far:      #7A3A18 (dark earth), #4A2810 (deep shadow)
  Scrub/bush:      #3A3A22 (dark olive), #2A3018 (near black-green)
  Tumbleweed:      #8A7848 (straw brown)
  Skull/bone:      #D8C8A0 (bleached)

Style keywords: WPA poster, gouache painting, flat color shapes, mural art, national park poster.
No soft shadows. No texture. No gradients. Pure flat paint color shapes only.`

// outlawFloorPanels defines 4 seamlessly-continuable floor tiles.
// These are GROUND STRIP textures — no sky, just the desert ground surface.
// Displayed as the bottom 220px of the screen. Hooves land at the top edge.
var outlawFloorPanels = []struct {
	Filename string
	Prompt   string
}{
	{
		Filename: "seamless_desert_floor",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Horizontal floor tile for side-scrolling mobile game.
This is the DESERT GROUND seen from the side — the strip of earth the characters run along.
Inspired by the lower ground portion of a flat geometric low-poly western desert illustration.

STRICT RULES:
- NO SKY. NO CLOUDS. NO SUN. NO MOUNTAINS. NO HORIZON VISIBLE. EARTH ONLY.
- The TOP EDGE of this image is the running surface — perfectly flat horizontal line.
- Characters' hooves touch exactly this top edge.

COMPOSITION top→bottom:
  TOP 10%: Running surface. Flat hard edge of terracotta earth (#C05828).
    3-4 tiny low angular scrub-bush shapes (#3A3A22) sitting right on the surface edge.
    2 small angular rock shards at the surface.
  MIDDLE 55%: Stacked flat-color earth strata bands.
    Band 1 (just below surface): Terracotta #C05828 — wide band.
    Band 2: Rust-orange #9A4220 — medium band.
    Band 3: Sand highlight #D4904A — thin bright stripe.
    Band 4: Dark rust #7A3A18 — wide band.
    Each band is a perfectly flat horizontal shape with sharp angular edges.
  BOTTOM 35%: Deep earth. Dark brown #4A2810 fading to near-black #2A1800.

Seamless tile: left and right edges match perfectly for infinite horizontal repeat.
NO gradients inside shapes. Hard flat polygon edges only. Low-poly vector style.`,
	},
	{
		Filename: "seamless_desert_floor2",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Horizontal floor tile, panel 2 of 4, side-scrolling game.
Must continue seamlessly from the reference image provided (left edge of this panel = right edge of panel 1).

STRICT RULES: NO SKY. NO CLOUDS. NO MOUNTAINS. EARTH ONLY. Top edge = running surface.

COMPOSITION top→bottom:
  TOP 10%: Running surface. Same flat terracotta level (#C05828) as reference left edge.
    FEATURE: Angular wooden signpost shape rising from ground, center-left.
    Bleached skull shape (#D8C8A0) mounted on top of post.
    One flat angular tumbleweed (#8A7848) on the ground surface, center.
    1-2 small rock shards.
  MIDDLE 55%: Flat strata bands matching reference color palette.
    Terracotta #C05828, rust #9A4220, sand #D4904A, dark rust #7A3A18 — horizontal band layers.
  BOTTOM 35%: Deep dark earth #4A2810 to #2A1800.

Right edge designed to flow into panel 3. Hard flat polygons only. No gradients.`,
	},
	{
		Filename: "seamless_desert_floor3",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Horizontal floor tile, panel 3 of 4, side-scrolling game.
Must continue seamlessly from the reference image provided (left edge = right edge of panel 2).

STRICT RULES: NO SKY. NO CLOUDS. NO MOUNTAINS. EARTH ONLY. Top edge = running surface.

COMPOSITION top→bottom:
  TOP 10%: Running surface. Same flat terracotta level as reference left edge.
    FEATURE: 2 tall saguaro cactus silhouettes rising from the ground surface.
    Cactus color: dark olive #3A3A22. Simplified geometric: vertical trunk + 2 arm branches.
    Hard angular flat-color polygon shapes only — no gradients, no outlines.
    2 small angular rock clusters at surface.
  MIDDLE 55%: Earth strata. Warmer orange tones this panel.
    Sand #D4904A prominent band. Terracotta #C05828 and rust #9A4220 beneath.
    Dark rust #7A3A18 lower.
  BOTTOM 35%: Deep dark earth #4A2810 to near-black #2A1800.

Right edge designed to flow into panel 4. Hard flat polygons only. No gradients.`,
	},
	{
		Filename: "seamless_desert_floor4",
		Prompt: outlawStyle + `

GROUND STRIP TEXTURE — 2048×512 pixels. Horizontal floor tile, panel 4 of 4, side-scrolling game.
Must continue seamlessly from the reference image provided (left edge = right edge of panel 3).
RIGHT EDGE must also be designed to loop back seamlessly into panel 1.

STRICT RULES: NO SKY. NO CLOUDS. NO MOUNTAINS. EARTH ONLY. Top edge = running surface.

COMPOSITION top→bottom:
  TOP 10%: Running surface. Bright hot desert sand #D4904A — sunbaked exposed earth.
    3 more saguaro cactus shapes continuing from left. Sparse desert feel.
    1 angular rock shard.
  MIDDLE 55%: Sunbaked earth strata. Bright orange-sand #D4904A dominant.
    Terracotta #C05828 and rust #9A4220 beneath. Transitioning back toward cooler rust on right.
  BOTTOM 35%: Deep dark earth #4A2810. Slightly cooler than panel 3.

Right edge transitions back toward cooler terracotta tones to loop back to panel 1.
Hard flat polygons only. No gradients. Low-poly vector style.`,
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

// ── H-SLOTS: Outlaw Gold symbol generation ───────────────────────────────────

const slotsSymbolStyle = `ART STYLE: Bold flat geometric illustration, vintage western casino poster style.
Pure white background, completely isolated symbol, clean crisp edges for background removal.
NO gradients inside any shape. NO drop shadows. NO 3D effects. NO photorealism.
Each shape = one solid flat color. Hard sharp edges between adjacent colors.
Premium casino slot game icon quality. Symbol centered, fills ~80% of frame.
Color palette: rich saturated western poster colors — deep reds, gold, warm brown, ivory.`

type slotSymbolItem struct {
	Filename string
	Prompt   string
}

var slotsSymbolPrompts = []slotSymbolItem{
	{
		Filename: "horseshoe",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A large lucky horseshoe. Thick bold U-shape, bright warm gold color. Two round nail holes on each arm. Simple bold outline style. Isolated on pure white background. Classic western luck symbol.",
	},
	{
		Filename: "whiskey",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A whiskey bottle. Dark brown bottle silhouette, amber flat-color liquid block inside. Round cork stopper. Small white rectangular label with a red diagonal stripe. Bold flat graphic shapes. Isolated on pure white background.",
	},
	{
		Filename: "bullet",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A single large bullet cartridge, pointing upward. Brass-gold cylindrical body, silver-grey pointed tip. Clean bold silhouette. Two flat-color shapes only: gold body + silver tip. Isolated on pure white background.",
	},
	{
		Filename: "badge",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A western sheriff badge, five-pointed star shape inside a circle. Bold thick outline, flat gold color. The star is solid gold with a slightly darker ring border. Engraved lines on star points. Simple bold iconic design. Isolated on pure white background.",
	},
	{
		Filename: "lantern",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: An old western oil lantern. Bold geometric trapezoid glass body, flat amber-orange glow color inside. Dark metal top handle and base, flat black shapes. Warm yellow glow flat disc behind lantern. Bold iconic silhouette. Isolated on pure white background.",
	},
	{
		Filename: "revolver",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A classic western revolver gun, pointing diagonally left-up. Bold side-view silhouette. Dark gunmetal grey flat color body, warm brown flat handle/grip. Cylinder visible as small circle. Bold iconic shape. Isolated on pure white background.",
	},
	{
		Filename: "cowboy_hat",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A classic cowboy hat, front view. Wide flat brim, tall crown with center crease, flat warm brown color. Simple bold silhouette shape. Decorative flat band strip in a slightly darker brown around base of crown. Isolated on pure white background.",
	},
	{
		Filename: "gold_bag",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A bulging moneybag filled with gold coins. Round plump bag shape, flat warm sandy-gold color. Tied at top with flat dark rope. Dollar sign or gold star embossed on front as a flat darker-gold shape. Bold iconic silhouette. Isolated on pure white background.",
	},
	{
		Filename: "dynamite",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A bundle of three dynamite sticks tied together. Bold cylindrical red sticks, flat red color. Wrapped with flat brown paper band around middle. Black fuse rope sticking up, curling slightly. Bold dramatic icon. Isolated on pure white background.",
	},
	{
		Filename: "outlaw",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A WANTED poster framed rectangle. Aged cream/ivory paper flat background. Bold black text 'WANTED' at top. Simple flat-color silhouette of a cowboy outlaw face/bust in center — dark hat, bandana mask covering lower face, dark coat. Red '$' symbol below. Bold vintage poster style. Isolated on pure white background.",
	},
	{
		Filename: "sheriff",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A sheriff's star badge with 'SHERIFF' text on a ribbon banner below. Large five-pointed star, flat gold color with dark outline. Bold engraved star points. Cream-colored ribbon banner beneath, bold black 'SHERIFF' lettering. Premium casino icon quality. Isolated on pure white background.",
	},
	{
		Filename: "wild",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A WILD bonus symbol for a slot game. Bold stylized text 'WILD' in large western-style letters. Each letter a bold flat gold color with dark outline. Surrounded by flat glowing rays/spikes in deep red and gold alternating. Like a gold star burst frame. Premium slot WILD symbol. Isolated on pure white background.",
	},
	{
		Filename: "scatter",
		Prompt:   slotsSymbolStyle + "\n\nSYMBOL: A SCATTER bonus symbol. A gold coin with a sheriff star stamped on its face. Round circle, bold flat gold color. Stamped star emblem in darker gold. Small flat rays/sparkles around the coin edge. Bold 'SCATTER' text in small bold letters below. Premium slot game icon. Isolated on pure white background.",
	},
}

// GenerateSlotSymbols handles POST /admin/v1/gemini/preset/slots-symbols.
func (h *Handler) GenerateSlotSymbols(c *gin.Context) {
	subDir := filepath.Join("h-slots", "outlaw-gold", "symbols")
	dir := filepath.Join(h.baseDir, subDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mkdir: " + err.Error()})
		return
	}

	results := make([]batchResult, len(slotsSymbolPrompts))
	for i, item := range slotsSymbolPrompts {
		imgBytes, _, err := h.client.GenerateImage(c.Request.Context(), item.Prompt)
		if err != nil {
			results[i] = batchResult{Filename: item.Filename, Error: err.Error()}
			continue
		}
		imgBytes, err = postProcess(imgBytes, true, 512)
		if err != nil {
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
			URL:      fmt.Sprintf("/assets/h-slots/outlaw-gold/symbols/%s.png", item.Filename),
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
