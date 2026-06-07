package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunAssign_Antigravity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "renkin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy docker.conf
	dockerConfPath := filepath.Join(tempDir, "docker.conf")
	err = os.WriteFile(dockerConfPath, []byte(`base_image = "ubuntu:24.04"`), 0644)
	if err != nil {
		t.Fatalf("failed to create docker.conf: %v", err)
	}

	// Create a dummy tool_list.toml with an MCP tool
	toolListPath := filepath.Join(tempDir, "tool_list.toml")
	err = os.WriteFile(toolListPath, []byte(`
[[tool]]
name = "test-tool"
type = "mcp"
mcp_config_gemini = '"test-tool": {"command": "test-cmd"}'
image = "test-image"
port = 8080
`), 0644)
	if err != nil {
		t.Fatalf("failed to create tool_list.toml: %v", err)
	}

	// Create a dummy llm.conf
	llmConfPath := filepath.Join(tempDir, "llm.conf")
	err = os.WriteFile(llmConfPath, []byte(`
cmd = "agy"
runtime_configs = [
    { source = "mcp_config.json", target = "/root/.gemini/config/mcp_config.json" }
]
home_mounts = [
    { host_dir = ".renkin/gemini", container_dir = "/root/.gemini" }
]
`), 0644)
	if err != nil {
		t.Fatalf("failed to create llm.conf: %v", err)
	}

	// Run assign command
	cmd := &cobra.Command{}
	llmPath = llmConfPath // Use direct path
	dockerPath = dockerConfPath
	toolsPath = []string{toolListPath}
	skillsPath = ""

	err = runAssign(cmd, []string{tempDir})
	if err != nil {
		t.Fatalf("runAssign failed: %v", err)
	}

	// Verify home mount directory creation
	renkinDir := filepath.Join(tempDir, ".renkin", "gemini")
	if _, err := os.Stat(renkinDir); os.IsNotExist(err) {
		t.Errorf(".renkin/gemini directory was not created")
	}

	// Verify mcp_config.json generation
	mcpConfigPath := filepath.Join(tempDir, ".renkin", "conf", "mcp_config.json")
	if _, err := os.Stat(mcpConfigPath); os.IsNotExist(err) {
		t.Errorf("mcp_config.json was not generated in .renkin/conf")
	}

	content, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		t.Fatalf("failed to read mcp_config.json: %v", err)
	}
	if !strings.Contains(string(content), `"test-tool": {"command": "test-cmd"}`) {
		t.Errorf("mcp_config.json content is incorrect: %s", string(content))
	}

	// Verify GEMINI.md generation (agy uses GEMINI.md)
	geminiMdPath := filepath.Join(tempDir, ".renkin", "conf", "GEMINI.md")
	if _, err := os.Stat(geminiMdPath); os.IsNotExist(err) {
		t.Errorf("GEMINI.md was not generated in .renkin/conf for agy")
	}

	// Verify Dockerfile contains placement logic referencing /renkin-conf
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("failed to read Dockerfile: %v", err)
	}

	expectedSnippet := `if [ -f /renkin-conf/mcp_config.json ]; then
  mkdir -p /root/.gemini/config
  python3 -c 'import os, sys; print(os.path.expandvars(sys.stdin.read()))' < /renkin-conf/mcp_config.json > /root/.gemini/config/mcp_config.json
fi`
	if !strings.Contains(string(dockerfileContent), expectedSnippet) {
		t.Errorf("Dockerfile does not contain the expected placement logic for mcp_config.json (referenced /renkin-conf)")
	}

	// Verify Skill file placement in Dockerfile
	skillSnippet := `if [ -f /renkin-conf/GEMINI.md ]; then
  cp /renkin-conf/GEMINI.md /root/.gemini/GEMINI.md
fi`
	if !strings.Contains(string(dockerfileContent), skillSnippet) {
		t.Errorf("Dockerfile does not contain the expected skill file placement logic")
	}

	// Verify docker-compose.yml contains home mount
	dcPath := filepath.Join(tempDir, "docker-compose.yml")
	dcContent, err := os.ReadFile(dcPath)
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}
	if !strings.Contains(string(dcContent), "- ./.renkin/gemini:/root/.gemini") {
		t.Errorf("docker-compose.yml does not contain the expected home mount")
	}
}
