package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenkinAssignWithLLMPreset(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-llm-preset-test")
	defer os.RemoveAll(tmpDir)
	
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Prepare presets directory
	presetsDir := filepath.Join(tmpDir, "presets", "llms")
	os.MkdirAll(presetsDir, 0755)
	presetContent := `
cmd = "my-llm"
install = "RUN echo llm-installed"
`
	os.WriteFile(filepath.Join(presetsDir, "my-llm.toml"), []byte(presetContent), 0644)

	// Prepare tools presets so resolution doesn't fail
	os.MkdirAll(filepath.Join(tmpDir, "presets", "tools"), 0755)

	// Prepare fixture files
	dockerConf := `base_image = "ubuntu:24.04"
[[mount]]
host = "./workspace"
container = "/workspace"
`
	toolList := `[[tool]]
name = "dummy"
type = "shell"
install = "RUN echo dummy"
`

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(toolList), 0644)

	// Run renkin assign using the LLM preset name
	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--llm", "my-llm", // Use preset name
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	assignCmd.Dir = tmpDir
	output, err := assignCmd.CombinedOutput()
	assert.NoError(t, err, string(output))

	// Verify generated Dockerfile contains LLM install step
	df, err := os.ReadFile(filepath.Join(targetDir, "Dockerfile"))
	assert.NoError(t, err)
	assert.Contains(t, string(df), "RUN echo llm-installed")
}
