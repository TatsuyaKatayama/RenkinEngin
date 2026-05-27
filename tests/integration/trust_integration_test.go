package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenkinAssignTrust(t *testing.T) {
	// Build the binary
	tmpDir, _ := os.MkdirTemp("", "renkin-trust-test")
	defer os.RemoveAll(tmpDir)
	
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../" // Project root
	err := buildCmd.Run()
	assert.NoError(t, err)

	// Test Gemini
	targetDirGemini := filepath.Join(tmpDir, "target-gemini")
	assignGemini := exec.Command(binPath, "assign", targetDirGemini, "--llm", "gemini")
	assignGemini.Dir = "../../" // Need to access presets
	output, err := assignGemini.CombinedOutput()
	assert.NoError(t, err, string(output))

	composeGemini, _ := os.ReadFile(filepath.Join(targetDirGemini, "docker-compose.yml"))
	assert.Contains(t, string(composeGemini), "- GEMINI_TRUST_WORKSPACE=true")
	
	dockerfileGemini, _ := os.ReadFile(filepath.Join(targetDirGemini, "Dockerfile"))
	assert.Contains(t, string(dockerfileGemini), "git config --global --add safe.directory /workspace")

	// Test Codex
	targetDirCodex := filepath.Join(tmpDir, "target-codex")
	assignCodex := exec.Command(binPath, "assign", targetDirCodex, "--llm", "codex")
	assignCodex.Dir = "../../"
	output, err = assignCodex.CombinedOutput()
	assert.NoError(t, err, string(output))

	composeCodex, _ := os.ReadFile(filepath.Join(targetDirCodex, "docker-compose.yml"))
	assert.Contains(t, string(composeCodex), "- CODEX_TRUST_WORKSPACE=true")

	dockerfileCodex, _ := os.ReadFile(filepath.Join(targetDirCodex, "Dockerfile"))
	assert.Contains(t, string(dockerfileCodex), "git config --global --add safe.directory /workspace")
}
