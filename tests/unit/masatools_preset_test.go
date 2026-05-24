package unit

import (
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
