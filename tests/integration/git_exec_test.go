package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecGitConfigEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-git-test")
	defer os.RemoveAll(tmpDir)

	t.Setenv("GIT_USER_NAME", "TestUser")
	t.Setenv("GIT_USER_EMAIL", "testuser@example.com")

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

	presetPath := "../../presets/tools/git.toml"
	presetContent, err := os.ReadFile(presetPath)
	if err != nil {
		t.Fatalf("failed to read preset: %v", err)
	}

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), presetContent, 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	envContent, err := os.ReadFile(filepath.Join(targetDir, ".env"))
	assert.NoError(t, err)
	assert.Contains(t, string(envContent), "GIT_USER_NAME=\n")
	assert.Contains(t, string(envContent), "GIT_USER_EMAIL=\n")

	err = os.WriteFile(filepath.Join(targetDir, ".env"), []byte("GIT_USER_NAME=EnvFileUser\nGIT_USER_EMAIL=\n"), 0644)
	assert.NoError(t, err)

	buildComposeCmd := exec.Command("docker", "compose", "build", "--no-cache")
	buildComposeCmd.Dir = targetDir
	if out, err := buildComposeCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose build failed: %v\n%s", err, string(out))
	}

	startCmd := exec.Command(binPath, "start")
	startCmd.Dir = targetDir
	if out, err := startCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin start failed: %v\n%s", err, string(out))
	}
	defer func() {
		downCmd := exec.Command(binPath, "stop")
		downCmd.Dir = targetDir
		downCmd.Run()
	}()

	execGit := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "bash", "-ic", "git config --global --get user.name && git config --global --get user.email")
	execGit.Dir = targetDir
	out, err := execGit.CombinedOutput()

	assert.NoError(t, err, string(out))
	assert.Contains(t, string(out), "EnvFileUser")
	assert.Contains(t, string(out), "testuser@example.com")
}
