package unit

import (
	"strings"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestDiscordMCPPresetResolution(t *testing.T) {
	presetsDir := "../../presets/tools"

	tl := config.ToolList{
		Tools: []config.Tool{
			{Preset: "discord-mcp"},
		},
	}

	err := tl.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Len(t, tl.Tools, 1)

	tool := tl.Tools[0]
	assert.Equal(t, "discord-mcp", tool.Name)
	assert.Equal(t, "mcp", tool.Type)
	assert.Equal(t, "renkin/discord-mcp:fork", tool.Image)
	assert.Equal(t, "https://github.com/TatsuyaKatayama/discord-mcp.git", tool.BuildContext)
	assert.Equal(t, 8085, tool.Port)
	assert.Equal(t, "/actuator/health", tool.HealthPath)
	assert.Len(t, tool.Mounts, 1)
	assert.Equal(t, "./workspace", tool.Mounts[0].Host)
	assert.Equal(t, "/workspace", tool.Mounts[0].Container)
	assert.Contains(t, tool.Environment, "SPRING_PROFILES_ACTIVE=http")
	assert.Contains(t, tool.Environment, "DISCORD_TOKEN")
	assert.Contains(t, tool.Environment, "DISCORD_GUILD_ID")
	assert.Contains(t, tool.Environment, "DISCORD_CHANNEL_ID")
	assert.Contains(t, tool.Environment, "DISCORD_MCP_URL")
	assert.Contains(t, tool.Environment, "DISCORD_MCP_STATE_FILE=/workspace/.discord_mcp_state.properties")
	assert.Contains(t, tool.Environment, "DISCORD_LAST_MESSAGE_ID")
	assert.Contains(t, tool.MCPConfigCodex, "[mcp_servers.discord]")
	assert.Contains(t, tool.MCPConfigCodex, `url = "http://discord-mcp:8085/mcp"`)
	assert.Contains(t, tool.MCPConfigGemini, `"discord"`)
	assert.Contains(t, tool.Instructions, "read_new_messages")
	assert.Contains(t, tool.Instructions, "get_user_id_by_name")
	assert.Contains(t, tool.Instructions, "<@sender_id>")
	assert.Contains(t, tool.Instructions, "Do not use the message ID of the last message you sent")
	assert.Contains(t, tool.Instructions, "message_reference")
}

func TestDiscordMCPPresetEnvGeneration(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/discord-mcp.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		ToolList: list,
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "DISCORD_TOKEN=")
	assert.Contains(t, env, "DISCORD_GUILD_ID=")
	assert.Contains(t, env, "DISCORD_CHANNEL_ID=")
	assert.Contains(t, env, "DISCORD_MCP_URL=")
	assert.Contains(t, env, "DISCORD_LAST_MESSAGE_ID=")
	assert.NotContains(t, env, "DISCORD_MCP_STATE_FILE=/workspace/.discord_mcp_state.properties=")
	assert.NotContains(t, env, "SPRING_PROFILES_ACTIVE=http=")
}

func TestDiscordMCPPresetDockerCompose(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/discord-mcp.toml")
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
	assert.Contains(t, compose, "discord-mcp:")
	assert.Contains(t, compose, "build:")
	assert.Contains(t, compose, "context: https://github.com/TatsuyaKatayama/discord-mcp.git")
	assert.Contains(t, compose, "image: renkin/discord-mcp:fork")
	assert.Contains(t, compose, `- "8085:8085"`)
	assert.Contains(t, compose, "- SPRING_PROFILES_ACTIVE=http")
	assert.Contains(t, compose, "- DISCORD_TOKEN")
	assert.Contains(t, compose, "- DISCORD_MCP_STATE_FILE=/workspace/.discord_mcp_state.properties")
	assert.Contains(t, compose, "volumes:")
	assert.Contains(t, compose, "- ./workspace:/workspace")
	assert.Contains(t, compose, "depends_on:")
	assert.Contains(t, compose, "condition: service_healthy")
	assert.Contains(t, compose, `wget", "-qO-", "http://localhost:8085/actuator/health"`)

	serviceIndex := strings.Index(compose, "  discord-mcp:")
	if !assert.NotEqual(t, -1, serviceIndex) {
		return
	}
	llmAgentBlock := compose[:serviceIndex]
	assert.NotContains(t, llmAgentBlock, "DISCORD_TOKEN")
	assert.NotContains(t, llmAgentBlock, "DISCORD_GUILD_ID")
	assert.NotContains(t, llmAgentBlock, "DISCORD_CHANNEL_ID")
}
