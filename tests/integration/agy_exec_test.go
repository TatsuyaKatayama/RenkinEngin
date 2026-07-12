package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecAgyPreset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-agy-test")
	defer os.RemoveAll(tmpDir)
	t.Setenv("GEMINI_API_KEY", "test-api-key")

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build renkin: %v\n%s", err, string(out))
	}

	dockerConf := `base_image = "ubuntu:24.04"
[[mount]]
host = "./workspace"
container = "/workspace"
`

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(`[[tool]]
name = "agy-mcp"
type = "shell"
install = "RUN echo installed"
mcp_config_gemini = '"agy-mcp": {"command": "echo"}'
`), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--llm", "agy",
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	assignCmd.Dir = "../../"
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	envContent, err := os.ReadFile(filepath.Join(targetDir, ".env"))
	assert.NoError(t, err)
	assert.Contains(t, string(envContent), "GEMINI_API_KEY=")

	dfContent, err := os.ReadFile(filepath.Join(targetDir, "Dockerfile"))
	assert.NoError(t, err)
	assert.Contains(t, string(dfContent), "renkin-generate-llm-config")
	assert.Contains(t, string(dfContent), "/root/.gemini/config/mcp_config.json")
	assert.NotContains(t, string(dfContent), "/root/.gemini/GEMINI.md")

	composeContent, err := os.ReadFile(filepath.Join(targetDir, "docker-compose.yml"))
	assert.NoError(t, err)
	assert.Contains(t, string(composeContent), "- ./.renkin/gemini:/root/.gemini")
	assert.Contains(t, string(composeContent), "- ./.renkin/conf:/renkin-conf:ro")

	buildComposeCmd := exec.Command("docker", "compose", "build")
	buildComposeCmd.Dir = targetDir
	if out, err := buildComposeCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose build failed: %v\n%s", err, string(out))
	}

	defer func() {
		downCmd := exec.Command(binPath, "kaiko", "--yes")
		downCmd.Dir = targetDir
		downCmd.Run()
	}()

	startCmd := exec.Command(binPath, "start", "--cmd", `agy --version`)
	startCmd.Dir = targetDir
	startOut, err := startCmd.CombinedOutput()
	assert.NoError(t, err, string(startOut))
	assert.Contains(t, string(startOut), "agy")

	execCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "bash", "-c", "test -f /root/.gemini/config/mcp_config.json && grep -q 'agy-mcp' /root/.gemini/config/mcp_config.json && grep -q 'echo' /root/.gemini/config/mcp_config.json && test -f /workspace/GEMINI.md")
	execCmd.Dir = targetDir
	out, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(out))
}
