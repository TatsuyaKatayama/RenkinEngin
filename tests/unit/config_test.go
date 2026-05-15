package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLLMConfParse(t *testing.T) {
	input := `
cmd = "claude --dangerously-skip-permissions"
auth_mode = "api_key"
install = """
RUN curl -fsSL https://claude.ai/install.sh | sh
"""
`
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "llm.conf")
	os.WriteFile(path, []byte(input), 0644)

	conf, err := config.LoadLLMConf(path)
	assert.NoError(t, err)
	assert.Equal(t, "claude --dangerously-skip-permissions", conf.Cmd)
	assert.Equal(t, "api_key", conf.AuthMode)
	assert.Contains(t, conf.Install, "curl -fsSL")
}

func TestToolListParse(t *testing.T) {
	input := `
[[tool]]
name = "openfoam"
type = "shell"
install = "RUN apt-get install -y openfoam2412"

[[tool]]
name = "lightrag"
type = "mcp"
image = "lightrag/server:latest"
port = 8080
`
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "tool_list.toml")
	os.WriteFile(path, []byte(input), 0644)

	list, err := config.LoadToolList(path)
	assert.NoError(t, err)
	assert.Len(t, list.Tools, 2)
	assert.Equal(t, "shell", list.Tools[0].Type)
	assert.Equal(t, "mcp", list.Tools[1].Type)
	assert.Equal(t, 8080, list.Tools[1].Port)
}

func TestLLMTypeIdentification(t *testing.T) {
	tests := []struct {
		cmd      string
		expected string
	}{
		{"claude --dangerously-skip-permissions", "claude"},
		{"gemini", "gemini"},
		{"codex -c", "codex"},
		{"opencode", "opencode"},
	}

	for _, tt := range tests {
		conf := config.LLMConf{Cmd: tt.cmd}
		llmType, err := conf.GetType()
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, llmType)
	}
}
