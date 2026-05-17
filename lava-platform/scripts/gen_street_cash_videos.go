//go:build ignore

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiKey  = "AIzaSyAsIIFzZeIm5nUgyFJ68wLnwEapCxfxN-Q"
	veoURL  = "https://generativelanguage.googleapis.com/v1beta/models/veo-3.0-generate-001:predictLongRunning"
	veoBase = "https://generativelanguage.googleapis.com/v1beta"
	outDir  = "frontend/public/video/street-cash"
)

func styleBase() string {
	return `Cinematic 3D animation, pure black background (no environment, no floor, no shadows on background). ` +
		`Ultra-premium street luxury / hip-hop luxury aesthetic. ` +
		`Gold, platinum, chrome materials with cinematic lighting. ` +
		`Smooth 60fps quality 3D render, 5 seconds duration. `
}

var winVideos = []struct {
	name   string
	scene  string
}{
	{
		"win-0",
		`Two chrome casino dice tumble in from above, spinning, and land perfectly showing sixes. ` +
			`On landing: explosion of silver sparks and gold coin burst outward in 360°. ` +
			`Text "×2" materializes in chrome 3D letters above the dice with silver glow aura. ` +
			`Silver confetti rains down. Dramatic flash of white light on landing impact.`,
	},
	{
		"win-1",
		`Rose gold aviator sunglasses spin into frame from the right with motion blur, then snap to center perfectly. ` +
			`Lenses catch dramatic studio light — blue lens flare shoots outward. ` +
			`Gold and rose-pink sparks burst from the frame. ` +
			`Text "×5" appears in rose gold metallic 3D letters with prismatic glow. ` +
			`Subtle rainbow light refracts from lenses.`,
	},
	{
		"win-2",
		`A premium luxury sneaker drops from above and hits center with impact, sending shockwave outward. ` +
			`Explosion of gold coins and neon green light burst upward on impact. ` +
			`Gold dust cloud rises dramatically. ` +
			`Text "×10" appears in gold 3D block letters with neon green outline glow. ` +
			`Street energy — dynamic, powerful, urban.`,
	},
	{
		"win-3",
		`Thick Cuban gold chain swings into frame from left, heavy links catching light individually, settles with satisfying weight. ` +
			`Massive shower of gold coins rains from top of frame. ` +
			`Intense golden light rays burst outward from the chain. ` +
			`Text "×20" appears in massive embossed gold 3D letters with blinding golden glow aura. ` +
			`Gold dust fills the air.`,
	},
	{
		"win-4",
		`Luxury gold wristwatch with diamond bezel rotates slowly into frame, stopping perfectly at center. ` +
			`Diamond bezel catches light — rainbow caustic patterns burst outward in all directions. ` +
			`Explosion of diamond-like particles and gold sparks. ` +
			`Text "×50" appears in diamond-studded gold 3D letters, incredibly bright. ` +
			`Prismatic rainbow light dances across the scene.`,
	},
	{
		"win-5",
		`Matte black luxury car key fob drops into frame, lands with a satisfying click. ` +
			`Blue LED buttons glow intensely, pulse outward. ` +
			`Purple and blue neon light explosion erupts behind the key fob. ` +
			`Sports car silhouette briefly appears in purple neon light. ` +
			`Text "×70" appears in purple chrome 3D letters with electric blue glow. ` +
			`Purple and gold sparks rain down.`,
	},
	{
		"win-6",
		`Black platinum credit card flies in fast with motion blur, rotates 720°, then freezes perfectly at slight angle in center. ` +
			`Holographic stripe explodes into rainbow light beams shooting in all directions. ` +
			`Platinum and gold light pillars rise from the card. ` +
			`The most intense win animation: "×100" appears in enormous platinum 3D letters with blinding white-gold glow. ` +
			`Gold coin tsunami erupts, filling the entire frame. ` +
			`Supreme jackpot energy — this is the ultimate win.`,
	},
}

type veoReq struct {
	Instances  []veoInst  `json:"instances"`
	Parameters veoParams  `json:"parameters"`
}
type veoInst struct{ Prompt string `json:"prompt"` }
type veoParams struct {
	AspectRatio     string `json:"aspectRatio"`
	DurationSeconds int    `json:"durationSeconds"`
}

func generateVideo(name, scene string) error {
	prompt := styleBase() + scene
	body, _ := json.Marshal(veoReq{
		Instances:  []veoInst{{Prompt: prompt}},
		Parameters: veoParams{AspectRatio: "9:16", DurationSeconds: 5},
	})
	resp, err := http.Post(fmt.Sprintf("%s?key=%s", veoURL, apiKey), "application/json", strings.NewReader(string(body)))
	if err != nil { return err }
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		sz := len(raw); if sz > 500 { sz = 500 }
		return fmt.Errorf("veo start %d: %s", resp.StatusCode, string(raw[:sz]))
	}

	var op struct {
		Name  string `json:"name"`
		Done  bool   `json:"done"`
		Error *struct{ Message string `json:"message"` } `json:"error"`
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
	}
	json.Unmarshal(raw, &op)
	fmt.Printf("  operation: %s\n", op.Name)

	opName := strings.TrimPrefix(op.Name, "/")
	pollURL := fmt.Sprintf("%s/%s?key=%s", veoBase, opName, apiKey)
	client := &http.Client{Timeout: 60 * time.Second}

	for i := 0; i < 120; i++ {
		time.Sleep(10 * time.Second)
		r, err := client.Get(pollURL)
		if err != nil { fmt.Printf("  poll err: %v\n", err); continue }
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		json.Unmarshal(b, &op)
		if op.Error != nil { return fmt.Errorf("veo: %s", op.Error.Message) }
		if !op.Done { fmt.Printf("  polling %ds...\n", (i+1)*10); continue }
		if op.Response == nil || op.Response.GenerateVideoResponse == nil {
			return fmt.Errorf("veo: empty response")
		}
		samples := op.Response.GenerateVideoResponse.GeneratedSamples
		if len(samples) == 0 { return fmt.Errorf("veo: no samples") }
		vid := samples[0].Video
		var data []byte
		if vid.BytesBase64Encoded != "" {
			data, _ = base64.StdEncoding.DecodeString(vid.BytesBase64Encoded)
		} else if vid.URI != "" {
			sep := "?"; if strings.Contains(vid.URI, "?") { sep = "&" }
			dr, _ := client.Get(vid.URI + sep + "key=" + apiKey)
			data, _ = io.ReadAll(dr.Body)
			dr.Body.Close()
		}
		out := fmt.Sprintf("%s/%s.mp4", outDir, name)
		if err := os.WriteFile(out, data, 0644); err != nil { return err }
		fmt.Printf("  saved %s (%dKB)\n", out, len(data)/1024)
		return nil
	}
	return fmt.Errorf("timeout")
}

func main() {
	os.MkdirAll(outDir, 0755)
	for _, v := range winVideos {
		fmt.Printf("\nGenerating %s...\n", v.name)
		if err := generateVideo(v.name, v.scene); err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		}
		time.Sleep(5 * time.Second)
	}
	fmt.Println("\nAll videos done!")
}
