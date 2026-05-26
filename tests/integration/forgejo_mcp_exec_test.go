package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecForgejoMCPPreset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-forgejo-mcp-test")
	defer os.RemoveAll(tmpDir)

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
preset = "forgejo-mcp"
`), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	assignCmd.Dir = "../../"
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	envContent, err := os.ReadFile(filepath.Join(targetDir, ".env"))
	assert.NoError(t, err)
	assert.Contains(t, string(envContent), "FORGEJO_URL=\n")
	assert.Contains(t, string(envContent), "FORGEJO_ACCESS_TOKEN=\n")
	assert.Contains(t, string(envContent), "FORGEJO_USER_AGENT=\n")

	buildComposeCmd := exec.Command("docker", "compose", "build", "--no-cache")
	buildComposeCmd.Dir = targetDir
	if out, err := buildComposeCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose build failed: %v\n%s", err, string(out))
	}

	upCmd := exec.Command("docker", "compose", "up", "-d")
	upCmd.Dir = targetDir
	if out, err := upCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose up failed: %v\n%s", err, string(out))
	}
	defer func() {
		downCmd := exec.Command("docker", "compose", "down")
		downCmd.Dir = targetDir
		downCmd.Run()
	}()

	execCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "bash", "-c", "renkin-generate-llm-config && git --version && command -v forgejo-mcp && forgejo-mcp --help >/dev/null && test -f /root/.codex/config.toml && test -f /root/.gemini/settings.json && grep -q '/usr/local/bin/forgejo-mcp' /root/.codex/config.toml && grep -q '/usr/local/bin/forgejo-mcp' /root/.gemini/settings.json && grep -q 'https://codeberg.org' /root/.codex/config.toml && grep -q 'https://codeberg.org' /root/.gemini/settings.json")
	execCmd.Dir = targetDir
	out, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(out))
}
