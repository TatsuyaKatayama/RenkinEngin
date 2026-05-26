package unit

import (
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestWorkspaceTrustEnvironment(t *testing.T) {
	// Test Gemini
	cfgGemini := config.Config{
		LLM: &config.LLMConf{
			Cmd: "gemini",
		},
	}
	composeGemini, err := generator.GenerateDockerCompose(cfgGemini)
	assert.NoError(t, err)
	assert.Contains(t, composeGemini, "- GEMINI_TRUST_WORKSPACE=true")

	// Test Codex
	cfgCodex := config.Config{
		LLM: &config.LLMConf{
			Cmd: "codex",
		},
	}
	composeCodex, err := generator.GenerateDockerCompose(cfgCodex)
	assert.NoError(t, err)
	assert.Contains(t, composeCodex, "- CODEX_TRUST_WORKSPACE=true")
}

func TestLLMPresetTrustCommands(t *testing.T) {
	// Gemini Preset
	lConfGemini, err := config.LoadLLMConf("../../presets/llms/gemini.toml")
	assert.NoError(t, err)
	assert.Contains(t, lConfGemini.Install, "git config --global --add safe.directory /workspace")

	// Codex Preset
	lConfCodex, err := config.LoadLLMConf("../../presets/llms/codex.toml")
	assert.NoError(t, err)
	assert.Contains(t, lConfCodex.Install, "git config --global --add safe.directory /workspace")
	assert.NotContains(t, lConfCodex.Install, "/root/.codex/config.toml")
}

func TestGitToolPresetTrustCommand(t *testing.T) {
	tList, err := config.LoadToolList("../../presets/tools/git.toml")
	assert.NoError(t, err)
	assert.Len(t, tList.Tools, 1)
	assert.Contains(t, tList.Tools[0].Install, "git config --global --add safe.directory /workspace")
}
