package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecCodexVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-codex-test")
	defer os.RemoveAll(tmpDir)
	t.Setenv("OPENAI_API_KEY", "test-api-key")

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
	// Read the real Codex preset
	presetPath := "../../presets/llms/codex.toml"
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
	buildCmd2 := exec.Command("docker", "compose", "build")
	buildCmd2.Dir = targetDir
	if out, err := buildCmd2.CombinedOutput(); err != nil {
		t.Fatalf("docker compose build failed: %v\n%s", err, string(out))
	}

	startCmd := exec.Command(binPath, "start", "--cmd", "true")
	startCmd.Dir = targetDir
	if out, err := startCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin start failed: %v\n%s", err, string(out))
	}
	defer func() {
		downCmd := exec.Command(binPath, "end")
		downCmd.Dir = targetDir
		downCmd.Run()
	}()

	// Verify installation
	execCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "bash", "-c", "codex --version && test -f /root/.codex/config.toml")
	execCmd.Dir = targetDir

	output, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(output))
	assert.Contains(t, string(output), "codex")
}
