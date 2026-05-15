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

func TestLLMConfParseError(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name  string
		input string
	}{
		{"No cmd", `auth_mode = "api_key"`},
		{"Invalid auth_mode", `cmd = "claude"
auth_mode = "ssh"`},
		{"TOML syntax error", `cmd = `},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, "llm_err.conf")
			os.WriteFile(path, []byte(tt.input), 0644)
			_, err := config.LoadLLMConf(path)
			assert.Error(t, err)
		})
	}
}

func TestDockerConfParse(t *testing.T) {
	input := `
[[mount]]
host = "./workspace"
container = "/workspace"

[[mount]]
host = "./data"
container = "/data"
`
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "docker.conf")
	os.WriteFile(path, []byte(input), 0644)

	conf, err := config.LoadDockerConf(path)
	assert.NoError(t, err)
	assert.Equal(t, "ubuntu:24.04", conf.BaseImage) // Default
	assert.Len(t, conf.Mounts, 2)
	assert.Equal(t, "./workspace", conf.Mounts[0].Host)
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
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			conf := config.LLMConf{Cmd: tt.cmd}
			llmType, err := conf.GetType()
			if tt.expected == "unknown" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, llmType)
			}
		})
	}
}

func TestSkillFileName(t *testing.T) {
	tests := []struct {
		cmd      string
		expected string
	}{
		{"claude", "CLAUDE.md"},
		{"gemini", "GEMINI.md"},
		{"codex", "AGENTS.md"},
		{"opencode", "AGENTS.md"},
	}

	for _, tt := range tests {
		conf := config.LLMConf{Cmd: tt.cmd}
		name, err := conf.GetSkillFileName()
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, name)
	}
}
