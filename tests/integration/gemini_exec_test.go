package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecGeminiVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-gemini-test")
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	dockerConf := `base_image = "ubuntu:24.04"
[[mount]]
host = "./workspace"
container = "/workspace"
`
	// Read the real Gemini preset
	presetPath := "../../presets/llms/gemini.toml"
	presetContent, err := os.ReadFile(presetPath)
	if err != nil {
		t.Fatalf("failed to read preset: %v", err)
	}

	toolList := `[[tool]]
name = "minimal-tool"
type = "shell"
install = "RUN echo installed"
`

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "llm.conf"), presetContent, 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(toolList), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--llm", filepath.Join(fixtureDir, "llm.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	// Build and Start
	if os.Getenv("CI") != "" {
		buildCmd := exec.Command("docker", "compose", "build", "--no-cache")
		buildCmd.Dir = targetDir
		if out, err := buildCmd.CombinedOutput(); err != nil {
			t.Fatalf("docker compose build failed: %v\n%s", err, string(out))
		}
	}

	upCmd := exec.Command("docker", "compose", "up", "-d")
	upCmd.Dir = targetDir
	if out, err := upCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose up failed: %v\n%s", err, string(out))
	}
	
	// Wait a bit
	time.Sleep(2 * time.Second)

	// Verify installation
	execCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "gemini", "--help")
	execCmd.Dir = targetDir
	
	output, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(output))
	assert.Contains(t, string(output), "Commands")

	// Cleanup
	downCmd := exec.Command("docker", "compose", "down")
	downCmd.Dir = targetDir
	downCmd.Run()
}
