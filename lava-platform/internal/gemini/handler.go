package gemini

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
	baseDir string // directory where generated assets are saved
}

// NewHandler creates a Gemini asset generation handler.
// baseDir is relative to the working directory, e.g. "frontend/dist/assets/bubble".
func NewHandler(apiKey, baseDir string) *Handler {
	return &Handler{
		client:  NewClient(apiKey),
		baseDir: baseDir,
	}
}

type generateReq struct {
	Prompt   string `json:"prompt" binding:"required"`
	Filename string `json:"filename" binding:"required"` // e.g. "bubble_stage_3" (no extension)
	Game     string `json:"game"`                        // e.g. "bubble" — subfolder
}

// Generate handles POST /admin/v1/gemini/generate
// Calls Gemini with the given prompt, saves the image to baseDir/game/filename.png,
// and returns the public asset URL.
func (h *Handler) Generate(c *gin.Context) {
	var req generateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Sanitise filename and game subfolder
	req.Filename = strings.ToLower(strings.TrimSpace(req.Filename))
	req.Game = strings.ToLower(strings.TrimSpace(req.Game))
	if req.Game == "" {
		req.Game = "bubble"
	}
	if !safeFilename.MatchString(req.Filename) || !safeFilename.MatchString(req.Game) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename and game must be lowercase alphanumeric/dash/underscore"})
		return
	}

	imgBytes, mimeType, err := h.client.GenerateImage(c.Request.Context(), req.Prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("gemini: %v", err)})
		return
	}

	ext := ".png"
	if strings.Contains(mimeType, "jpeg") {
		ext = ".jpg"
	}

	dir := filepath.Join(h.baseDir, req.Game)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mkdir: " + err.Error()})
		return
	}

	outPath := filepath.Join(dir, req.Filename+ext)
	if err := os.WriteFile(outPath, imgBytes, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write: " + err.Error()})
		return
	}

	assetURL := fmt.Sprintf("/assets/%s/%s%s", req.Game, req.Filename, ext)
	c.JSON(http.StatusOK, gin.H{
		"url":  assetURL,
		"path": outPath,
		"size": len(imgBytes),
		"mime": mimeType,
	})
}
