package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenkinAssignWithPreset(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-preset-int-test")
	defer os.RemoveAll(tmpDir)
	
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Prepare presets directory
	presetsDir := filepath.Join(tmpDir, "presets", "tools")
	os.MkdirAll(presetsDir, 0755)
	presetContent := `
[[tool]]
name = "resolved-preset"
type = "shell"
install = "RUN echo preset-installed"
`
	os.WriteFile(filepath.Join(presetsDir, "my-preset.toml"), []byte(presetContent), 0644)

	// Prepare fixture files
	dockerConf := `base_image = "ubuntu:24.04"
[[mount]]
host = "./workspace"
container = "/workspace"
`
	toolList := `[[tool]]
preset = "my-preset"
`

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(toolList), 0644)

	// Run renkin assign from the directory containing presets/
	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	assignCmd.Dir = tmpDir // Run from tmpDir so it finds ./presets/tools
	output, err := assignCmd.CombinedOutput()
	assert.NoError(t, err, string(output))

	// Verify generated Dockerfile
	df, err := os.ReadFile(filepath.Join(targetDir, "Dockerfile"))
	assert.NoError(t, err)
	assert.Contains(t, string(df), "RUN echo preset-installed")
}
