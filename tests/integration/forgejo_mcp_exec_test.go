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
	t.Setenv("GIT_USER_NAME", "test-user")
	t.Setenv("GIT_USER_EMAIL", "test@example.com")
	t.Setenv("OPENAI_API_KEY", "test-api-key")
	t.Setenv("FORGEJO_URL", "https://codeberg.org")
	t.Setenv("FORGEJO_ACCESS_TOKEN", "test-access-token")
	t.Setenv("FORGEJO_USER_AGENT", "test-user-agent")

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
		"--llm", "presets/llms/codex.toml",
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	assignCmd.Dir = "../../"
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	envContent, err := os.ReadFile(filepath.Join(targetDir, ".env"))
	assert.NoError(t, err)
	assert.Contains(t, string(envContent), "OPENAI_API_KEY=")
	assert.Contains(t, string(envContent), "FORGEJO_URL=\n")
	assert.Contains(t, string(envContent), "FORGEJO_ACCESS_TOKEN=\n")
	assert.Contains(t, string(envContent), "FORGEJO_USER_AGENT=\n")

	dfContent, err := os.ReadFile(filepath.Join(targetDir, "Dockerfile"))
	assert.NoError(t, err)
	assert.Contains(t, string(dfContent), "renkin-generate-llm-config")
	assert.Contains(t, string(dfContent), "/root/.codex/config.toml")
	assert.Contains(t, string(dfContent), "/root/.gemini/settings.json")

	composeContent, err := os.ReadFile(filepath.Join(targetDir, "docker-compose.yml"))
	assert.NoError(t, err)
	assert.Contains(t, string(composeContent), "- ./.renkin/codex:/root/.codex")
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

	startCmd := exec.Command(binPath, "start", "--cmd", `bash -lc "codex mcp list --json"`)
	startCmd.Dir = targetDir
	startOut, err := startCmd.CombinedOutput()
	assert.NoError(t, err, string(startOut))
	assert.Contains(t, string(startOut), "forgejo")

	execCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "bash", "-c", "test -f /root/.codex/config.toml && grep -q '/usr/local/bin/forgejo-mcp' /root/.codex/config.toml && grep -q 'https://codeberg.org' /root/.codex/config.toml")
	execCmd.Dir = targetDir
	out, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(out))
}
