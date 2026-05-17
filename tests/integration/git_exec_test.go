package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestDockerExecGitVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-git-test")
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "renkin")
	exec.Command("go", "build", "-o", binPath, "./cmd/renkin").Run()

	dockerConf := `base_image = "ubuntu:24.04"`
	
	// Prepare preset
	presetsDir := filepath.Join(tmpDir, "presets", "tools")
	os.MkdirAll(presetsDir, 0755)
	utils.CopyFile("../../presets/tools/git.toml", filepath.Join(presetsDir, "git.toml"))

	toolList := `[[tool]]
preset = "git"
`
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(targetDir, "tool_list.toml"), []byte(toolList), 0644)

	assignCmd := exec.Command(binPath, "assign", targetDir)
	assignCmd.Dir = tmpDir
	assignCmd.Run()

	composeFile := filepath.Join(targetDir, "docker-compose.yml")
	exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "--build").Run()
	
	// Verify Git
	execGit := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", "llm-agent", "git", "--version")
	out, err := execGit.CombinedOutput()
	assert.NoError(t, err, string(out))
	assert.Contains(t, string(out), "git version")

	exec.Command("docker", "compose", "-f", composeFile, "down").Run()
}
