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
		Filename: "grow", Duration: 8, AspectRatio: "9:16",
		Prompt: "A magical pink bubblegum bubble self-inflating autonomously, floating in dark deep-purple cosmic candy space. Starts tiny like a marble, grows steadily and smoothly to fill the entire frame. Perfectly round glossy neon-pink surface, bright white specular highlight shifting as it grows, translucent bubble walls glowing hot-pink from within. No people, no hands, no characters, no text. Smooth continuous inflation motion. Neon-pink and violet ambient light. Cinematic premium mobile game animation. Clean dark background.",
	},
	{
		Filename: "pop", Duration: 5, AspectRatio: "9:16",
		Prompt: "A massive glossy neon-pink bubblegum bubble dramatically exploding in slow motion. Micro-cracks form on the surface then catastrophic burst — hot-pink gum shards and sticky translucent strands fly outward radially in all directions, bright shockwave ring of light, vivid neon flash. No people, no hands, no characters, no text. Dark purple-black background. Cinematic slow-motion explosion. Premium mobile game visual effect.",
	},
}

// outlawVeoStyle is the mandatory art direction for all outlaw character videos.
const outlawVeoStyle = `
Art style: flat geometric low-poly vector illustration. Travel poster / WPA mural aesthetic.
Bold flat color blocking. Geometric simplified western characters and horses.
Zero photorealism. Sharp angular shapes defining silhouettes. Limited color palette.
NO white outlines. NO bright edge highlights. NO glow effects. NO halos around shapes.
All shape edges defined ONLY by adjacent flat color contrast — no white border lines.
Background: PURE PITCH BLACK (#000000) — required for luma-key compositing in game engine.
No background scenery whatsoever. Character and horse only, on pure black.
The character must appear as a clean flat-color silhouette with NO white stripes or edge artifacts.`

// outlawVideoPresets defines the 3 video assets for the Outlaw Escape game visual rebuild.
var outlawVideoPresets = []struct {
	Filename    string
	Duration    int
	AspectRatio string
	Prompt      string
}{
	{
		Filename: "outlaw_run", Duration: 5, AspectRatio: "9:16",
		Prompt: `Seamlessly looping side-view gallop animation.
SUBJECT: Outlaw character riding BLACK horse, moving RIGHT across the frame.
CHARACTER: Dark bandana across the lower face, worn wide-brim outlaw hat with feather.
Dusty duster/trail coat. Dark shirt. Side profile, full body visible.
HORSE: Pure black stallion. Full side-profile gallop cycle — all four hooves in motion.
Powerful muscular silhouette. Mane and tail flowing back from speed.
MOTION: Seamless loop, continuous full-gallop. No jump. No weapon. Pure escape running.
Camera: Fixed side view. Character centered horizontally. Full horse + rider visible.
` + outlawVeoStyle,
	},
	{
		Filename: "sheriff_run_shoot", Duration: 5, AspectRatio: "9:16",
		Prompt: `Seamlessly looping side-view gallop animation.
SUBJECT: Sheriff/lawman character riding WHITE horse, moving RIGHT across the frame.
CHARACTER: Wide-brim sheriff hat with visible badge on chest. Duster coat with vest detail.
Side profile, full body visible.
HORSE: Pure white stallion. Full side-profile gallop cycle — all four hooves in motion.
Flowing white mane and tail.
ACTION: Right arm extended forward holding and firing a revolver. Left hand holds reins.
Occasional muzzle flash at barrel tip. Determined pursuing posture.
MOTION: Seamless loop, full-gallop pursuit. Shooting while riding.
Camera: Fixed side view. Full horse + rider visible.
` + outlawVeoStyle,
	},
	{
		Filename: "outlaw_crash", Duration: 5, AspectRatio: "9:16",
		Prompt: `Dramatic crash/capture animation. Non-looping, plays once.
SEQUENCE: Outlaw on black horse is caught by sheriff on white horse from behind.
The black horse stumbles and collapses forward. The outlaw rider is thrown dramatically.
Both characters visible: sheriff pulling alongside from right, outlaw falling left-forward.
TIMING: 0-1s: horses at full gallop, sheriff closing gap. 1-2s: impact, horse collapse begins.
2-3s: outlaw rider thrown forward, coat spreading, dramatic fall moment.
No comedy. Serious cinematic crash. Dusty impact.
Camera: Fixed side view. Both characters visible.
` + outlawVeoStyle,
	},
}

// GenerateOutlawVideos handles POST /admin/v1/veo/preset/outlaw
func (h *Handler) GenerateOutlawVideos(c *gin.Context) {
	type result struct {
		Filename string `json:"filename"`
		URL      string `json:"url,omitempty"`
		Error    string `json:"error,omitempty"`
	}
	results := make([]result, len(outlawVideoPresets))
	for i, p := range outlawVideoPresets {
		res, err := h.generateOne(c, p.Prompt, p.Filename, "outlaw", p.Duration, p.AspectRatio)
		if err != nil {
			results[i] = result{Filename: p.Filename, Error: err.Error()}
		} else {
			results[i] = result{Filename: p.Filename, URL: res["url"].(string)}
		}
	}
	c.JSON(http.StatusOK, results)
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

// winCelebrationStyle is mandatory art direction for win celebration videos.
const winCelebrationStyle = `
Art style: flat geometric bold graphic design. Western outlaw casino aesthetic.
Premium game UI animation. Dark background with dramatic lighting and particles.
NO photorealism. Bold geometric shapes and flat colors with strong contrast.
Cinematic motion graphics feel. Inspired by Pragmatic Play premium slot art direction.`

// winCelebrationPresets defines the 4 win-tier celebration video assets.
var winCelebrationPresets = []struct {
	Filename    string
	Duration    int
	AspectRatio string
	Prompt      string
}{
	{
		Filename: "win_nice", Duration: 5, AspectRatio: "9:16",
		Prompt: `Premium casino win celebration animation for a western-themed mobile slot game.
TIER: NICE WIN — warm elegant golden celebration.
BACKGROUND: Deep black/dark brown (#0b0804) fading to warm amber at center.
PARTICLES: Gold coins and dollar signs raining downward from the top, gently rotating.
COINS: Flat geometric disc shapes in gold (#f7d44c) and brass (#b8860b). Tumbling and spinning.
ATMOSPHERE: Warm golden dust particles drifting upward. Subtle radial golden glow at center.
MOTION: 5 seconds. First 1s: particle system activates and builds. 2-4s: full golden shower. 5s: graceful fade-out.
MOOD: Satisfying, elegant, premium — like finding a gold nugget in a river.
NO text or typography in the video. Pure visual FX only.
` + winCelebrationStyle,
	},
	{
		Filename: "win_big", Duration: 6, AspectRatio: "9:16",
		Prompt: `Premium casino win celebration animation for a western-themed mobile slot game.
TIER: BIG WIN — explosive red and gold western saloon prestige.
BACKGROUND: Deep black to dramatic deep red (#400000) burst from center outward.
OPENING: Screen flash of warm white light, then dramatic red/gold burst.
PARTICLES: Gold coins, paper banknotes, playing card suits (spades/hearts) flying radially outward from center.
SHOCKWAVE: A visible ring of golden energy expanding outward from center at 0.3s.
ADDITIONAL FX: Sparks and brass-colored fragments trailing from coin impacts.
Subtle revolver chamber spin graphic in background (dark, barely visible).
MOTION: 6 seconds. 0-0.3s: intense flash. 0.3-2s: explosive radial particle burst. 2-5s: golden shower settling. 5-6s: fade.
MOOD: Powerful, explosive, premium — like a bank vault blasting open.
NO text or typography in the video. Pure visual FX only.
` + winCelebrationStyle,
	},
	{
		Filename: "win_mega", Duration: 7, AspectRatio: "9:16",
		Prompt: `Premium casino win celebration animation for a western-themed mobile slot game.
TIER: MEGA WIN — cinematic full-screen western jackpot spectacle.
BACKGROUND: Black to deep purple (#1a0040) with golden rays fanning outward from a central burst.
OPENING: Massive golden shockwave expanding from center, filling the frame. Screen-filling flash.
PARTICLES: Huge burst of gold bars, coins, gemstones, and dollar bags flying outward.
STEAM FX: Thick locomotive-style steam jets erupting from both sides of frame, lit from behind in gold.
SUNBURST: Geometric golden rays radiating from center like a train headlight breaking through.
GOLD RAIN: After initial burst, steady heavy golden shower for the middle section.
MOTION: 7 seconds. 0-0.5s: white flash + massive shockwave. 0.5-1.5s: explosion peak. 1.5-5s: gold shower + rays. 5-7s: fade.
MOOD: Massive, cinematic, awe-inspiring — like a locomotive loaded with treasure crashing through.
NO text or typography. Pure visual FX only.
` + winCelebrationStyle,
	},
	{
		Filename: "win_epic", Duration: 8, AspectRatio: "9:16",
		Prompt: `Premium casino win celebration animation for a western-themed mobile slot game.
TIER: EPIC / TRAIN HEIST JACKPOT — legendary highest-tier spectacle.
BACKGROUND: Pure black to extreme multicolor burst — gold, emerald, crimson rays erupting from center.
OPENING: The most extreme visual impact: full white flash, then the most dramatic explosion of all tiers.
VAULT BREACH: Geometric vault door shape shattering outward with metallic fragments.
GOLD FLOOD: Literal golden light flooding in from the breached vault — fills lower half of frame.
PARTICLES: Maximum density — gold bars, coins, bounty scrolls, sheriff badges, treasure chests all exploding outward.
CROWN FX: A geometric gold crown shape momentarily appears and shatters at the climax.
RAINBOW SPARKLES: Brief multicolored sparkle storm in final 2 seconds.
MOTION: 8 seconds. 0-0.3s: white-out. 0.3-1s: vault breach FX. 1-4s: maximum gold flood and particles. 4-6s: climax sparkles. 6-8s: fade.
MOOD: Transcendent, legendary — the most spectacular win possible.
NO text or typography. Pure visual FX only.
` + winCelebrationStyle,
	},
}

// GenerateWinCelebration handles POST /admin/v1/veo/preset/win-celebration
func (h *Handler) GenerateWinCelebration(c *gin.Context) {
	type result struct {
		Filename string `json:"filename"`
		URL      string `json:"url,omitempty"`
		Error    string `json:"error,omitempty"`
	}
	results := make([]result, len(winCelebrationPresets))
	for i, p := range winCelebrationPresets {
		res, err := h.generateOne(c, p.Prompt, p.Filename, "h-slots/outlaw-gold", p.Duration, p.AspectRatio)
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
