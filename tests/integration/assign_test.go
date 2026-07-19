package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenkinAssign(t *testing.T) {
	// Build the binary first
	tmpDir, _ := os.MkdirTemp("", "renkin-int-test")
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../" // Run from project root
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Prepare fixture files
	dockerConf := `base_image = "ubuntu:24.04"
[[mount]]
host = "./workspace"
container = "/workspace"
`
	llmConf := `cmd = "codex"
install = "RUN echo installed"
`
	toolList := `[[tool]]
name = "openfoam"
type = "shell"
install = "RUN echo openfoam"
`
	skills := "# Test skills"

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "llm.conf"), []byte(llmConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(toolList), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "skills.md"), []byte(skills), 0644)

	// Run renkin assign
	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--llm", filepath.Join(fixtureDir, "llm.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
		"--skills", filepath.Join(fixtureDir, "skills.md"),
	)
	output, err := assignCmd.CombinedOutput()
	assert.NoError(t, err, string(output))

	// Verify generated files
	assert.FileExists(t, filepath.Join(targetDir, "Dockerfile"))
	assert.FileExists(t, filepath.Join(targetDir, "docker-compose.yml"))
	assert.FileExists(t, filepath.Join(targetDir, ".env"))
	assert.FileExists(t, filepath.Join(targetDir, ".renkin_metadata.toml"))

	// Verify content
	df, _ := os.ReadFile(filepath.Join(targetDir, "Dockerfile"))
	assert.Contains(t, string(df), "FROM ubuntu:24.04")
	assert.Contains(t, string(df), "RUN echo openfoam")

	env, _ := os.ReadFile(filepath.Join(targetDir, ".env"))
	assert.NotContains(t, string(env), "GEMINI_API_KEY=")
}

func TestRenkinAssignWithDiscourseMCPPreset(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-discourse-preset-int-test")
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--llm", "codex",
		"--tools", "discourse-mcp",
	)
	assignCmd.Dir = "../../"
	output, err := assignCmd.CombinedOutput()
	assert.NoError(t, err, string(output))

	compose, err := os.ReadFile(filepath.Join(targetDir, "docker-compose.yml"))
	assert.NoError(t, err)
	assert.Contains(t, string(compose), "discourse-mcp:")
	assert.Contains(t, string(compose), "image: node:24")
	assert.Contains(t, string(compose), `- "8086:3000"`)
	assert.Contains(t, string(compose), "http://localhost:3000/health")

	env, err := os.ReadFile(filepath.Join(targetDir, ".env"))
	assert.NoError(t, err)
	assert.Contains(t, string(env), "DISCOURSE_BASE_URL=")
	assert.Contains(t, string(env), "DISCOURSE_API_KEY=")
	assert.Contains(t, string(env), "DISCOURSE_CATEGORY_ID=")
}
