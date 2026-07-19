package unit

import (
	"strings"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestDiscourseMCPPresetResolution(t *testing.T) {
	presetsDir := "../../presets/tools"

	tl := config.ToolList{
		Tools: []config.Tool{
			{Preset: "discourse-mcp"},
		},
	}

	err := tl.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Len(t, tl.Tools, 1)

	tool := tl.Tools[0]
	assert.Equal(t, "discourse-mcp", tool.Name)
	assert.Equal(t, "mcp", tool.Type)
	assert.Equal(t, "node:24", tool.Image)
	assert.Equal(t, 3000, tool.Port)
	assert.Equal(t, 8086, tool.HostPort)
	assert.Equal(t, "/health", tool.HealthPath)
	assert.Contains(t, tool.Command, "@discourse/mcp@latest")
	assert.Contains(t, tool.Command, "--transport http")
	assert.Contains(t, tool.Command, "--profile /tmp/discourse-mcp-profile.json")
	assert.Contains(t, tool.Environment, "DISCOURSE_BASE_URL")
	assert.Contains(t, tool.Environment, "DISCOURSE_API_KEY")
	assert.Contains(t, tool.Environment, "DISCOURSE_API_USERNAME=system")
	assert.Contains(t, tool.Environment, "DISCOURSE_CATEGORY_ID")
	assert.Contains(t, tool.Environment, "DISCOURSE_TOPIC_ID")
	assert.Contains(t, tool.Environment, "DISCOURSE_BOT_USER_ID")
	assert.Contains(t, tool.Environment, "DISCOURSE_MCP_URL")
	assert.Contains(t, tool.MCPConfigCodex, "[mcp_servers.discourse]")
	assert.Contains(t, tool.MCPConfigCodex, `url = "http://discourse-mcp:3000/mcp"`)
	assert.Contains(t, tool.MCPConfigGemini, `"discourse"`)
	assert.Contains(t, tool.Instructions, "discourse_filter_topics")
	assert.Contains(t, tool.Instructions, "discourse_read_topic")
	assert.Contains(t, tool.Instructions, "discourse_read_post")
	assert.Contains(t, tool.Instructions, "discourse_create_post")
	assert.Contains(t, tool.Instructions, "renkin:resolved")
}

func TestDiscourseMCPPresetEnvGeneration(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/discourse-mcp.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		ToolList: list,
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "DISCOURSE_BASE_URL=")
	assert.Contains(t, env, "DISCOURSE_API_KEY=")
	assert.Contains(t, env, "DISCOURSE_CATEGORY_ID=")
	assert.Contains(t, env, "DISCOURSE_TOPIC_ID=")
	assert.Contains(t, env, "DISCOURSE_BOT_USER_ID=")
	assert.Contains(t, env, "DISCOURSE_MCP_URL=")
	assert.NotContains(t, env, "DISCOURSE_API_USERNAME=system=")
}

func TestDiscourseMCPPresetDockerCompose(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/discourse-mcp.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		Docker:   config.DockerConf{BaseImage: "ubuntu:24.04"},
		LLM:      &config.LLMConf{Cmd: "codex"},
		ToolList: list,
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "discourse-mcp:")
	assert.Contains(t, compose, "image: node:24")
	assert.Contains(t, compose, "command: >")
	assert.Contains(t, compose, "npx -y @discourse/mcp@latest")
	assert.Contains(t, compose, `- "8086:3000"`)
	assert.Contains(t, compose, "- DISCOURSE_BASE_URL")
	assert.Contains(t, compose, "- DISCOURSE_API_KEY")
	assert.Contains(t, compose, "- DISCOURSE_API_USERNAME=system")
	assert.Contains(t, compose, "- DISCOURSE_CATEGORY_ID")
	assert.Contains(t, compose, "- DISCOURSE_TOPIC_ID")
	assert.Contains(t, compose, "- DISCOURSE_BOT_USER_ID")
	assert.Contains(t, compose, "- DISCOURSE_MCP_URL")
	assert.Contains(t, compose, "depends_on:")
	assert.Contains(t, compose, "condition: service_healthy")
	assert.Contains(t, compose, `wget", "-qO-", "http://localhost:3000/health"`)

	serviceIndex := strings.Index(compose, "  discourse-mcp:")
	if !assert.NotEqual(t, -1, serviceIndex) {
		return
	}
	llmAgentBlock := compose[:serviceIndex]
	assert.NotContains(t, llmAgentBlock, "env_file: .env")
	assert.NotContains(t, llmAgentBlock, "DISCOURSE_API_KEY")
	assert.NotContains(t, llmAgentBlock, "DISCOURSE_BASE_URL")
}
