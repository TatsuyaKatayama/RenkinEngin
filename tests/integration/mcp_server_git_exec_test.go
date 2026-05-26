package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecMCPServerGitPreset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-mcp-server-git-test")
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
preset = "mcp-server-git"
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

	execCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "bash", "-c", "renkin-generate-llm-config && git --version && command -v mcp-server-git && mcp-server-git --help >/dev/null && test -f /root/.codex/config.toml && grep -q '/usr/local/bin/mcp-server-git' /root/.codex/config.toml")
	execCmd.Dir = targetDir
	out, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(out))
}
