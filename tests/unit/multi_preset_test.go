package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestResolvePresetsScenarios(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-scenarios-test")
	defer os.RemoveAll(tmpDir)
	presetsDir := filepath.Join(tmpDir, "presets")
	os.MkdirAll(presetsDir, 0755)

	os.WriteFile(filepath.Join(presetsDir, "p1.toml"), []byte("[[tool]]\nname=\"tool-p1\"\ntype=\"shell\"\ninstall=\"RUN p1\"\nenvironment=[\"VAR1\"]"), 0644)
	os.WriteFile(filepath.Join(presetsDir, "p2.toml"), []byte("[[tool]]\nname=\"tool-p2\"\ntype=\"shell\"\ninstall=\"RUN p2\"\nenvironment=[\"VAR2\"]"), 0644)

	t.Run("Preset1 + Preset2 (Normal)", func(t *testing.T) {
		tl := config.ToolList{Tools: []config.Tool{{Preset: "p1"}, {Preset: "p2"}}}
		err := tl.ResolvePresets(presetsDir)
		assert.NoError(t, err)
		assert.Len(t, tl.Tools, 2)
		assert.Equal(t, "tool-p1", tl.Tools[0].Name)
		assert.Equal(t, "tool-p2", tl.Tools[1].Name)
	})

	t.Run("Preset1 + Preset1 (Duplicate Error)", func(t *testing.T) {
		tl := config.ToolList{Tools: []config.Tool{{Preset: "p1"}, {Preset: "p1"}}}
		err := tl.ResolvePresets(presetsDir)
		assert.Error(t, err)
	})

	t.Run("Preset1 + Direct (Multiple)", func(t *testing.T) {
		tl := config.ToolList{
			Tools: []config.Tool{
				{Preset: "p1"},
				{Name: "direct", Type: "shell", Install: "RUN direct", Environment: []string{"VAR3"}},
			},
		}
		err := tl.ResolvePresets(presetsDir)
		assert.NoError(t, err)
		assert.Len(t, tl.Tools, 2)
		assert.Equal(t, "tool-p1", tl.Tools[0].Name)
		assert.Equal(t, "direct", tl.Tools[1].Name)
	})

	t.Run("Preset1 + Install (Override)", func(t *testing.T) {
		tl := config.ToolList{Tools: []config.Tool{{Preset: "p1", Install: "RUN p1-override"}}}
		err := tl.ResolvePresets(presetsDir)
		assert.NoError(t, err)
		assert.Equal(t, "RUN p1-override", tl.Tools[0].Install)
	})

    t.Run("Direct + Direct", func(t *testing.T) {
		tl := config.ToolList{
			Tools: []config.Tool{
				{Name: "d1", Type: "shell", Install: "RUN d1"},
				{Name: "d2", Type: "shell", Install: "RUN d2"},
			},
		}
		err := tl.ResolvePresets(presetsDir)
		assert.NoError(t, err)
		assert.Len(t, tl.Tools, 2)
	})
}

func TestEnvGenerationMultiTool(t *testing.T) {
	cfg := config.Config{
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{Name: "t1", Environment: []string{"VAR1"}},
				{Name: "t2", Environment: []string{"VAR2"}},
			},
		},
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "VAR1=")
	assert.Contains(t, env, "VAR2=")
}
