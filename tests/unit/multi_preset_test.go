package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestResolveMultiplePresets(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-multi-preset-test")
	defer os.RemoveAll(tmpDir)

	presetsDir := filepath.Join(tmpDir, "presets")
	os.MkdirAll(presetsDir, 0755)

	// Create two presets
	os.WriteFile(filepath.Join(presetsDir, "tool-a.toml"), []byte("[[tool]]\nname=\"a\"\ntype=\"shell\"\ninstall=\"RUN a\""), 0644)
	os.WriteFile(filepath.Join(presetsDir, "tool-b.toml"), []byte("[[tool]]\nname=\"b\"\ntype=\"shell\"\ninstall=\"RUN b\""), 0644)

	// Tool list using multiple presets
	toolList := config.ToolList{
		Tools: []config.Tool{
			{Preset: "tool-a"},
			{Preset: "tool-b"},
			{Name: "c", Type: "shell", Install: "RUN c"},
		},
	}

	err := toolList.ResolvePresets(presetsDir)
	assert.NoError(t, err)
	assert.Len(t, toolList.Tools, 3)
	assert.Equal(t, "a", toolList.Tools[0].Name)
	assert.Equal(t, "b", toolList.Tools[1].Name)
	assert.Equal(t, "c", toolList.Tools[2].Name)
}
