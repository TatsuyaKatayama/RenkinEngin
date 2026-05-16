package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecOpenModelicaHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-om-test")
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
	// Read the real OpenModelica preset
	presetPath := "../../presets/tools/openmodelica410.toml"
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

	// Build and Start
	if os.Getenv("CI") != "" {
		buildCmd = exec.Command("docker", "compose", "build", "--no-cache")
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
	
	// Execute omc --help using absolute path
	execCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "/usr/bin/omc", "--help")
	execCmd.Dir = targetDir
	
	output, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(output))
	assert.Contains(t, string(output), "Usage")

	// Cleanup
	endCmd := exec.Command(binPath, "end")
	endCmd.Dir = targetDir
	endCmd.Run()
}
