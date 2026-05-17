//go:build ignore

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	gemURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-image:generateContent"
	apiKey = "AIzaSyAsIIFzZeIm5nUgyFJ68wLnwEapCxfxN-Q"
	outDir = "frontend/public/assets/street-cash"
)

func styleBase() string {
	return `3D rendered luxury slot machine game asset. ` +
		`Style: ultra-premium photorealistic 3D, hyper-volumetric, rich materials with subsurface scattering, ` +
		`street luxury / hip-hop luxury aesthetic — gold, platinum, chrome, glossy black. ` +
		`Composition: perfectly centered in square frame, object fills 80% of canvas, no clipping. ` +
		`Lighting: dramatic cinematic studio lighting from upper-left, strong specular highlights on metal/chrome surfaces, ` +
		`deep rich shadows emphasizing 3D depth, subtle gold rim light from below. ` +
		`Background: PURE BLACK — absolutely no background, no floor, no environment, no shadows on BG. ` +
		`Output: square image, AAA mobile slot machine icon quality, ultra-sharp edges, no motion blur. `
}

var symbols = []struct {
	name   string
	object string
}{
	{"sym-0", "a pair of chrome casino dice showing sixes, mirror-polished chrome surfaces, deep gold inlaid pips catching light, one die slightly angled showing depth, luxury casino aesthetic"},
	{"sym-1", "rose gold aviator sunglasses, oversized luxury designer frames in rose gold metal, dark gradient lenses with subtle sky reflection, polished chrome hinges, premium designer quality"},
	{"sym-2", "ultra-premium high-top luxury sneaker side profile, white and gold colorway, gold metallic sole with carbon fiber details, neon green swoosh, premium leather texture with visible stitching, gold metallic lace tips"},
	{"sym-3", "thick Cuban link gold chain necklace, massive heavy 18k yellow gold links, each link individually mirror-polished with diamond-cut facets, hanging in slight curve to show depth, diamond-encrusted clasp"},
	{"sym-4", "luxury wristwatch, thick gold case with diamond-set bezel, deep matte black dial, dauphine gold hands at 10:10, gold baton hour markers, date window, small seconds subdial, exhibition caseback partially visible"},
	{"sym-5", "sleek matte black luxury car key fob, premium soft-touch black body, chrome trim band around edges, embossed gold H logo center, three backlit buttons with soft blue LED glow, miniature sports car silhouette reflected in glossy surface"},
	{"sym-6", "ultra-premium black platinum credit card shown at slight angle, matte black card surface, 'H-GAMES' text in raised brushed platinum letters, gold EMV chip, holographic stripe showing rainbow iridescence, embossed card numbers in gold, Visa-style wave logo in platinum"},
}

type gemReq struct {
	Contents         []gemContent `json:"contents"`
	GenerationConfig gemConfig    `json:"generationConfig"`
}
type gemContent struct{ Parts []gemPart `json:"parts"` }
type gemPart struct{ Text string `json:"text"` }
type gemConfig struct{ ResponseModalities []string `json:"responseModalities"` }

func generate(name, object string) error {
	prompt := styleBase() + "Object to render: " + object + "."
	body, _ := json.Marshal(gemReq{
		Contents:         []gemContent{{Parts: []gemPart{{Text: prompt}}}},
		GenerationConfig: gemConfig{ResponseModalities: []string{"IMAGE"}},
	})
	url := fmt.Sprintf("%s?key=%s", gemURL, apiKey)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		sz := len(raw); if sz > 400 { sz = 400 }
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw[:sz]))
	}
	var gr struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	json.Unmarshal(raw, &gr)
	for _, c := range gr.Candidates {
		for _, p := range c.Content.Parts {
			if p.InlineData != nil && p.InlineData.Data != "" {
				img, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
				if err != nil { return err }
				ext := "png"
				if p.InlineData.MimeType == "image/jpeg" { ext = "jpg" }
				path := fmt.Sprintf("%s/%s.%s", outDir, name, ext)
				if err := os.WriteFile(path, img, 0644); err != nil { return err }
				fmt.Printf("  saved %s (%dKB)\n", path, len(img)/1024)
				return nil
			}
		}
	}
	sz := len(raw); if sz > 300 { sz = 300 }
	return fmt.Errorf("no image in response: %s", string(raw[:sz]))
}

func main() {
	os.MkdirAll(outDir, 0755)
	os.MkdirAll("frontend/dist/assets/street-cash", 0755)
	for _, s := range symbols {
		fmt.Printf("Generating %s...\n", s.name)
		if err := generate(s.name, s.object); err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		}
		time.Sleep(3 * time.Second)
	}
	fmt.Println("Done!")
}
