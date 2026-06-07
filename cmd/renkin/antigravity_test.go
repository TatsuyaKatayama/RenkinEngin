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
    { source = "mcp_config.json", target = "/workspace/.agents/mcp_config.json" }
]
mcp_config_target = "/workspace/.agents/mcp_config.json"
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

	// Verify mcp_config.json generation
	mcpConfigPath := filepath.Join(tempDir, "workspace", "mcp_config.json")
	if _, err := os.Stat(mcpConfigPath); os.IsNotExist(err) {
		t.Errorf("mcp_config.json was not generated")
	}

	content, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		t.Fatalf("failed to read mcp_config.json: %v", err)
	}
	if !strings.Contains(string(content), `"test-tool": {"command": "test-cmd"}`) {
		t.Errorf("mcp_config.json content is incorrect: %s", string(content))
	}

	// Verify GEMINI.md generation (agy uses GEMINI.md)
	geminiMdPath := filepath.Join(tempDir, "workspace", "GEMINI.md")
	if _, err := os.Stat(geminiMdPath); os.IsNotExist(err) {
		t.Errorf("GEMINI.md was not generated for agy")
	}

	// Verify Dockerfile contains placement logic
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("failed to read Dockerfile: %v", err)
	}

	expectedSnippet := `if [ -f /workspace/mcp_config.json ]; then
  mkdir -p /workspace/.agents
  python3 -c 'import os, sys; print(os.path.expandvars(sys.stdin.read()))' < /workspace/mcp_config.json > /workspace/.agents/mcp_config.json
fi`
	if !strings.Contains(string(dockerfileContent), expectedSnippet) {
		t.Errorf("Dockerfile does not contain the expected placement logic for mcp_config.json")
	}
}
