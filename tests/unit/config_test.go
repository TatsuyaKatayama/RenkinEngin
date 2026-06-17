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
cmd = "gemini"
install = """
RUN curl -fsSL https://example.com/install.sh | sh
"""
`
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "llm.conf")
	os.WriteFile(path, []byte(input), 0644)

	conf, err := config.LoadLLMConf(path)
	assert.NoError(t, err)
	assert.Equal(t, "gemini", conf.Cmd)
	assert.Contains(t, conf.Install, "curl -fsSL")
}

func TestLLMConfParseError(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name  string
		input string
	}{
		{"No cmd", `install = "echo install"`},
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
	assert.NotContains(t, tool.Install, "/root/.codex/config.toml")
	assert.NotContains(t, tool.Install, "/root/.gemini/settings.json")
	assert.Contains(t, tool.MCPConfigCodex, `args = ["--repository", "/workspace"]`)
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
	assert.NotContains(t, tool.Install, "/root/.codex/config.toml")
	assert.NotContains(t, tool.Install, "/root/.gemini/settings.json")
	assert.Contains(t, tool.MCPConfigCodex, "${FORGEJO_URL:-https://codeberg.org}")
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
		{"gemini", "gemini"},
		{"agy", "agy"},
		{"codex -c", "codex"},
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

func TestResolveDuplicatePresets(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-dup-test")
	defer os.RemoveAll(tmpDir)

	presetsDir := filepath.Join(tmpDir, "presets")
	os.MkdirAll(presetsDir, 0755)

	// Create 'git' preset
	gitContent := `
[[tool]]
name = "git"
type = "shell"
install = "RUN apt-get install git"
`
	os.WriteFile(filepath.Join(presetsDir, "git.toml"), []byte(gitContent), 0644)

	// Create 'forgejo' preset which includes 'git'
	forgejoContent := `
[[tool]]
preset = "git"

[[tool]]
name = "forgejo"
type = "shell"
install = "RUN echo forgejo"
`
	os.WriteFile(filepath.Join(presetsDir, "forgejo.toml"), []byte(forgejoContent), 0644)

	// User specifies both 'git' and 'forgejo' (which includes 'git')
	toolList := config.ToolList{
		Tools: []config.Tool{
			{Preset: "git"},
			{Preset: "forgejo"},
		},
	}

	err := toolList.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Len(t, toolList.Tools, 2)
	assert.Equal(t, "git", toolList.Tools[0].Name)
	assert.Equal(t, "forgejo", toolList.Tools[1].Name)
}

func TestResolveDuplicateNestedPresetWithoutOuterName(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-nested-dup-test")
	defer os.RemoveAll(tmpDir)

	presetsDir := filepath.Join(tmpDir, "presets")
	os.MkdirAll(presetsDir, 0755)

	pythonPostContent := `
[[tool]]
name = "python-post"
type = "shell"
install = "RUN echo python"
`
	os.WriteFile(filepath.Join(presetsDir, "python-post.toml"), []byte(pythonPostContent), 0644)

	masatoolsDir := filepath.Join(presetsDir, "masatools")
	os.MkdirAll(masatoolsDir, 0755)
	masatoolsContent := `
[[tool]]
preset = "python-post"

[[tool]]
name = "masatools-mcp"
type = "shell"
install = "RUN echo masatools"
`
	os.WriteFile(filepath.Join(masatoolsDir, "tool.toml"), []byte(masatoolsContent), 0644)
	os.WriteFile(
		filepath.Join(masatoolsDir, "mcp_config_codex.toml"),
		[]byte("[mcp_servers.masatools]\ncommand = \"python\""),
		0644,
	)

	toolList := config.ToolList{
		Tools: []config.Tool{
			{Preset: "masatools"},
			{Preset: "python-post"},
		},
	}

	err := toolList.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Len(t, toolList.Tools, 2)
	assert.Equal(t, "python-post", toolList.Tools[0].Name)
	assert.Equal(t, "masatools-mcp", toolList.Tools[1].Name)
	assert.Empty(t, toolList.Tools[0].MCPConfigCodex)
	assert.Contains(t, toolList.Tools[1].MCPConfigCodex, "[mcp_servers.masatools]")
}

func TestCollectEnvKeys(t *testing.T) {
	cfg := config.Config{
		LLM: &config.LLMConf{
			Cmd: "gemini",
		},
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{Name: "t1", Environment: []string{"VAR1", "VAR2"}},
				{Name: "t2", Environment: []string{"VAR2", "VAR3"}},
			},
		},
	}
	keys := cfg.CollectEnvKeys()
	assert.Contains(t, keys, "GEMINI_API_KEY")
	assert.Contains(t, keys, "AGENT_ID")
	assert.Contains(t, keys, "VAR1")
	assert.Contains(t, keys, "VAR2")
	assert.Contains(t, keys, "VAR3")

	// Check uniqueness
	count := 0
	for _, k := range keys {
		if k == "VAR2" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestSkillFileName(t *testing.T) {
	tests := []struct {
		cmd      string
		expected string
	}{
		{"gemini", "GEMINI.md"},
		{"agy", "GEMINI.md"},
		{"codex", "AGENTS.md"},
	}

	for _, tt := range tests {
		conf := config.LLMConf{Cmd: tt.cmd}
		name, err := conf.GetSkillFileName()
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, name)
	}
}

func TestLLMConfParseWithLoopFields(t *testing.T) {
	input := `
cmd = "gemini"
loop_cmd = "gemini --session-id {session_id}"
restart_policy = "on-failure"
restart_delay = 5
log_dir = ".renkin/logs"
stdout_log = "stdout.log"
stderr_log = "stderr.log"
`
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "llm.conf")
	os.WriteFile(path, []byte(input), 0644)

	conf, err := config.LoadLLMConf(path)
	assert.NoError(t, err)
	assert.Equal(t, "gemini", conf.Cmd)
	assert.Equal(t, "gemini --session-id {session_id}", conf.LoopCmd)
	assert.Equal(t, "on-failure", conf.RestartPolicy)
	assert.Equal(t, 5, conf.RestartDelay)
	assert.Equal(t, ".renkin/logs", conf.LogDir)
	assert.Equal(t, "stdout.log", conf.StdoutLog)
	assert.Equal(t, "stderr.log", conf.StderrLog)
}

func TestRenderLoopTemplate(t *testing.T) {
	loopTemplate := `#!/bin/bash
AGENT_ID=${AGENT_ID:-default-agent}
echo "Starting True Renkin Loop Iteration for ${AGENT_ID}..."
{llm_cmd}
	`
	llmConf := &config.LLMConf{
		LoopCmd: `codex exec resume --skip-git-repo-check {session_id} "$(cat /renkin-conf/bot_prompt.md)"`,
	}

	rendered := config.RenderLoopTemplate(loopTemplate, llmConf, "/tmp/renkin/masa-agent")

	assert.Contains(t, rendered, "AGENT_ID=${AGENT_ID:-masa-agent}")
	assert.Contains(t, rendered, `codex exec resume --skip-git-repo-check "${AGENT_ID}-session" "$(cat /renkin-conf/bot_prompt.md)"`)
	assert.NotContains(t, rendered, "{llm_cmd}")
	assert.NotContains(t, rendered, "default-agent")
}

func TestMetadataSaveAndLoadWithLoopFields(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-test")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, ".renkin_metadata.toml")

	meta := config.Metadata{
		LLMCmd:        "gemini",
		EnvKeys:       []string{"KEY1"},
		LoopCmd:       "gemini --session-id {session_id}",
		RestartPolicy: "always",
		RestartDelay:  5,
		LogDir:        ".renkin/logs",
		StdoutLog:     "stdout.log",
		StderrLog:     "stderr.log",
	}

	err := config.SaveMetadata(path, meta)
	assert.NoError(t, err)

	var loaded config.Metadata
	err = config.LoadMetadata(path, &loaded)
	assert.NoError(t, err)

	assert.Equal(t, "gemini", loaded.LLMCmd)
	assert.Equal(t, []string{"KEY1"}, loaded.EnvKeys)
	assert.Equal(t, "gemini --session-id {session_id}", loaded.LoopCmd)
	assert.Equal(t, "always", loaded.RestartPolicy)
	assert.Equal(t, 5, loaded.RestartDelay)
	assert.Equal(t, ".renkin/logs", loaded.LogDir)
	assert.Equal(t, "stdout.log", loaded.StdoutLog)
	assert.Equal(t, "stderr.log", loaded.StderrLog)
}
