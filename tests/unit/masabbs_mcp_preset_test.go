package unit

import (
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestMasabbsMCPPresetResolution(t *testing.T) {
	presetsDir := "../../presets/tools"

	tl := config.ToolList{
		Tools: []config.Tool{
			{Preset: "masabbs-mcp"},
		},
	}

	err := tl.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Len(t, tl.Tools, 1)

	tool := tl.Tools[0]
	assert.Equal(t, "masabbs-mcp", tool.Name)
	assert.Equal(t, "shell", tool.Type)
	assert.Contains(t, tool.Environment, "MASABBS_BASE_URL")
	assert.Contains(t, tool.Environment, "MASABBS_TIMEOUT_MS")
	assert.Contains(t, tool.Install, "github.com/TatsuyaKatayama/masabbs-mcp.git#fc199eaed658a08897e53c399d68de614dd0f029")
	assert.Contains(t, tool.Startup, "[mcp_servers.masabbs-mcp]")
	assert.Contains(t, tool.Startup, `"masabbs-mcp"`)
	assert.Contains(t, tool.Startup, "http://host.docker.internal/api/v1")

	cfg := config.Config{
		ToolList: tl,
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "MASABBS_BASE_URL=")
	assert.Contains(t, env, "MASABBS_TIMEOUT_MS=")
}

func TestMasabbsMCPPresetDockerfileAndRuntimeConfig(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/masabbs-mcp.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		Docker:   config.DockerConf{BaseImage: "ubuntu:24.04"},
		LLM:      &config.LLMConf{Cmd: "codex"},
		ToolList: list,
	}

	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "npm install -g")
	assert.Contains(t, dockerfile, "masabbs-mcp.git#fc199eaed658a08897e53c399d68de614dd0f029")
	assert.Contains(t, dockerfile, "renkin-generate-llm-config")
	assert.Contains(t, dockerfile, "[mcp_servers.masabbs-mcp]")
	assert.Contains(t, dockerfile, "renkin_add_gemini_mcp_server")
}
