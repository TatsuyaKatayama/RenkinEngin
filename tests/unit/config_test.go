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

func TestResolvePresets(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-preset-test")
	defer os.RemoveAll(tmpDir)

	presetsDir := filepath.Join(tmpDir, "presets")
	os.MkdirAll(presetsDir, 0755)

	// Create a dummy preset
	presetContent := `
[[tool]]
name = "openfoam2512"
type = "shell"
install = "RUN echo foam-preset"
`
	os.WriteFile(filepath.Join(presetsDir, "openfoam2512.toml"), []byte(presetContent), 0644)

	// Tool list using the preset
	toolList := config.ToolList{
		Tools: []config.Tool{
			{Name: "my-foam", Preset: "openfoam2512"},
		},
	}

	err := toolList.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Equal(t, "my-foam", toolList.Tools[0].Name)
	assert.Equal(t, "shell", toolList.Tools[0].Type)
	assert.Equal(t, "RUN echo foam-preset", toolList.Tools[0].Install)

	// Test fallback name
	toolList2 := config.ToolList{
		Tools: []config.Tool{
			{Preset: "openfoam2512"},
		},
	}
	err = toolList2.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Equal(t, "openfoam2512", toolList2.Tools[0].Name)
}

func TestGitToolPreset(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/git.toml")
	assert.NoError(t, err)
	assert.Len(t, list.Tools, 1)

	tool := list.Tools[0]
	assert.Equal(t, "git", tool.Name)
	assert.Equal(t, "shell", tool.Type)
	assert.Contains(t, tool.Install, "apt-get install -y git")
	assert.ElementsMatch(t, []string{"GIT_USER_NAME", "GIT_USER_EMAIL"}, tool.Environment)
}

func TestMCPServerGitToolPreset(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/mcp-server-git.toml")
	assert.NoError(t, err)
	assert.Len(t, list.Tools, 2)

	assert.Equal(t, "git", list.Tools[0].Preset)

	tool := list.Tools[1]
	assert.Equal(t, "mcp-server-git", tool.Name)
	assert.Equal(t, "shell", tool.Type)
	assert.Contains(t, tool.Install, "uv pip install --system --break-system-packages mcp-server-git")
	assert.Contains(t, tool.Install, "ln -s /root/.local/bin/uv /usr/local/bin/uv")
	assert.Contains(t, tool.Install, "/root/.codex/config.toml")
	assert.Contains(t, tool.Install, "/root/.gemini/settings.json")
	assert.Contains(t, tool.Install, `args = ["--repository", "/workspace"]`)
}

func TestMCPServerGitToolPresetResolution(t *testing.T) {
	list := config.ToolList{Tools: []config.Tool{{Preset: "mcp-server-git"}}}
	err := list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)
	assert.Len(t, list.Tools, 2)
	assert.Equal(t, "git", list.Tools[0].Name)
	assert.Equal(t, "mcp-server-git", list.Tools[1].Name)
}

func TestForgejoMCPToolPreset(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/forgejo-mcp.toml")
	assert.NoError(t, err)
	assert.Len(t, list.Tools, 2)

	assert.Equal(t, "git", list.Tools[0].Preset)

	tool := list.Tools[1]
	assert.Equal(t, "forgejo-mcp", tool.Name)
	assert.Equal(t, "shell", tool.Type)
	assert.Contains(t, tool.Install, "git clone --depth 1 https://github.com/goern/forgejo-mcp.git")
	assert.Contains(t, tool.Install, "go build -o /usr/local/bin/forgejo-mcp .")
	assert.Contains(t, tool.Install, "/root/.codex/config.toml")
	assert.Contains(t, tool.Install, "/root/.gemini/settings.json")
	assert.Contains(t, tool.Install, "${FORGEJO_URL:-https://codeberg.org}")
	assert.ElementsMatch(t, []string{"FORGEJO_URL", "FORGEJO_ACCESS_TOKEN", "FORGEJO_USER_AGENT"}, tool.Environment)
}

func TestForgejoMCPToolPresetResolution(t *testing.T) {
	list := config.ToolList{Tools: []config.Tool{{Preset: "forgejo-mcp"}}}
	err := list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)
	assert.Len(t, list.Tools, 2)
	assert.Equal(t, "git", list.Tools[0].Name)
	assert.Equal(t, "forgejo-mcp", list.Tools[1].Name)
	assert.ElementsMatch(t, []string{"FORGEJO_URL", "FORGEJO_ACCESS_TOKEN", "FORGEJO_USER_AGENT"}, list.Tools[1].Environment)
}

func TestResolvePresetsWithInstructions(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-preset-instr-test")
	defer os.RemoveAll(tmpDir)

	presetsDir := filepath.Join(tmpDir, "presets")
	os.MkdirAll(presetsDir, 0755)

	// Create a dummy preset with instructions
	presetContent := `
[[tool]]
name = "python-post"
type = "shell"
install = "RUN echo python"
instructions = "Use python3."
`
	os.WriteFile(filepath.Join(presetsDir, "python-post.toml"), []byte(presetContent), 0644)

	// Tool list using the preset
	toolList := config.ToolList{
		Tools: []config.Tool{
			{Preset: "python-post"},
		},
	}

	err := toolList.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Equal(t, "python-post", toolList.Tools[0].Name)
	assert.Equal(t, "Use python3.", toolList.Tools[0].Instructions)
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
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, llmType)
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
