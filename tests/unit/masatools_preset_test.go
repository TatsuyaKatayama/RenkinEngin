package unit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestMasatoolsPresetResolution(t *testing.T) {
	// Root of RenkinEngin project
	presetsDir := "../../presets/tools"

	// Define a tool list that uses the masatools preset
	tl := config.ToolList{
		Tools: []config.Tool{
			{Preset: "masatools"},
		},
	}

	// Resolve presets (this will follow the 'preset = "python-post"' link)
	err := tl.ResolvePresets(presetsDir)
	assert.NoError(t, err)

	// Verify that both tools are resolved (python-post from base preset, masatools-mcp from masatools.toml)
	assert.Len(t, tl.Tools, 2)

	names := []string{tl.Tools[0].Name, tl.Tools[1].Name}
	assert.Contains(t, names, "python-post")
	assert.Contains(t, names, "masatools-mcp")

	// Verify environment variables are collected
	var masatoolsTool config.Tool
	for _, t := range tl.Tools {
		if t.Name == "masatools-mcp" {
			masatoolsTool = t
			break
		}
	}
	assert.Contains(t, masatoolsTool.Environment, "NATS_URL")
	assert.Contains(t, masatoolsTool.Environment, "API_URL")

	// Verify .env generation contains these variables
	cfg := config.Config{
		ToolList: tl,
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "NATS_URL=")
	assert.Contains(t, env, "API_URL=")
}

func TestMasatoolsPresetLoopMetadata(t *testing.T) {
	preset, err := config.LoadToolPreset("../../presets/tools", "masatools")
	assert.NoError(t, err)
	assert.Equal(t, "on-success", preset.RestartPolicy)
	assert.Equal(t, 5, preset.RestartDelay)
	assert.Contains(t, preset.BotPrompt, "Final JSON Contract")
	assert.Contains(t, preset.BotPrompt, `{"loop_status":"fatal","reason":"connectivity_failed"}`)
	assert.Contains(t, preset.Instructions, "masabbs as the source of truth")
}

func TestCodexBotLoopParsesFinalJsonContract(t *testing.T) {
	loopTemplate, err := os.ReadFile("../../presets/llms/codex/bot-loop.sh")
	assert.NoError(t, err)

	rendered := config.RenderLoopTemplate(
		string(loopTemplate),
		&config.LLMConf{LoopCmd: `printf '%s\n' '{"loop_status":"idle","reason":"no_task"}'`},
		"/tmp/renkin/codex-agent",
	)

	scriptPath := filepath.Join(t.TempDir(), "bot-loop.sh")
	assert.NoError(t, os.WriteFile(scriptPath, []byte(rendered), 0755))

	output, err := runShellScript(scriptPath)
	assert.NoError(t, err, output)
	assert.Contains(t, output, `Renkin loop final JSON: {"loop_status":"idle","reason":"no_task"}`)
}

func TestCodexBotLoopRequiresDiscordReplyToolForBoardSuccess(t *testing.T) {
	loopTemplate, err := os.ReadFile("../../presets/llms/codex/bot-loop.sh")
	assert.NoError(t, err)

	rendered := config.RenderLoopTemplate(
		string(loopTemplate),
		&config.LLMConf{LoopCmd: `printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"loop_status\":\"success\",\"reason\":\"task_completed\"}"}}'`},
		"/tmp/renkin/codex-agent",
	)

	scriptPath := filepath.Join(t.TempDir(), "bot-loop.sh")
	assert.NoError(t, os.WriteFile(scriptPath, []byte(rendered), 0755))

	output, err := runShellScriptWithEnv(scriptPath, append(os.Environ(), "RENKIN_BOARD_ITEM_ID=101"))
	assert.Error(t, err)
	assert.Contains(t, output, "no Discord reply tool completed")
}

func TestCodexBotLoopAcceptsDiscordReplyToolForBoardSuccess(t *testing.T) {
	loopTemplate, err := os.ReadFile("../../presets/llms/codex/bot-loop.sh")
	assert.NoError(t, err)

	rendered := config.RenderLoopTemplate(
		string(loopTemplate),
		&config.LLMConf{LoopCmd: `printf '%s\n' '{"type":"item.completed","item":{"type":"mcp_tool_call","server":"discord","tool":"send_message","status":"completed","error":null}}' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"loop_status\":\"success\",\"reason\":\"task_completed\"}"}}'`},
		"/tmp/renkin/codex-agent",
	)

	scriptPath := filepath.Join(t.TempDir(), "bot-loop.sh")
	assert.NoError(t, os.WriteFile(scriptPath, []byte(rendered), 0755))

	output, err := runShellScriptWithEnv(scriptPath, append(os.Environ(), "RENKIN_BOARD_ITEM_ID=101"))
	assert.NoError(t, err, output)
	assert.Contains(t, output, `Renkin loop final JSON: {"loop_status":"success","reason":"task_completed"}`)
}

func TestCodexBotLoopRequiresAuthForCodexCommand(t *testing.T) {
	loopTemplate, err := os.ReadFile("../../presets/llms/codex/bot-loop.sh")
	assert.NoError(t, err)

	rendered := config.RenderLoopTemplate(
		string(loopTemplate),
		&config.LLMConf{LoopCmd: `codex exec "unused"`},
		"/tmp/renkin/codex-agent",
	)

	scriptPath := filepath.Join(t.TempDir(), "bot-loop.sh")
	assert.NoError(t, os.WriteFile(scriptPath, []byte(rendered), 0755))

	output, err := runShellScriptWithEnv(scriptPath, withoutEnv(os.Environ(), "OPENAI_API_KEY"))
	assert.Error(t, err)
	assert.Contains(t, output, "Codex authentication is not configured in the container.")
	assert.Contains(t, output, "renkin auth codex")
	assert.Contains(t, output, "Or copy your host auth file outside the container")
	assert.NotContains(t, output, "mkdir -p .renkin/codex")
	assert.Contains(t, output, "cp ~/.codex/auth.json .renkin/codex/auth.json")
}

func runShellScript(path string) (string, error) {
	cmd := exec.Command("bash", path)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runShellScriptWithEnv(path string, env []string) (string, error) {
	cmd := exec.Command("bash", path)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func withoutEnv(env []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
