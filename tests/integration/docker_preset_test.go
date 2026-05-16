package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenkinAssignWithDefaultDockerPreset(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-docker-preset-test")
	defer os.RemoveAll(tmpDir)
	
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Prepare presets directory
	presetsDir := filepath.Join(tmpDir, "presets", "docker")
	os.MkdirAll(presetsDir, 0755)
	presetContent := `
base_image = "alpine:latest"
[[mount]]
host = "./p"
container = "/p"
`
	os.WriteFile(filepath.Join(presetsDir, "default.toml"), []byte(presetContent), 0644)

	// Target dir has no docker.conf
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)

	// Run renkin assign (should fallback to default preset)
	assignCmd := exec.Command(binPath, "assign", targetDir)
	assignCmd.Dir = tmpDir
	output, err := assignCmd.CombinedOutput()
	assert.NoError(t, err, string(output))

	// Verify generated Dockerfile
	df, err := os.ReadFile(filepath.Join(targetDir, "Dockerfile"))
	assert.NoError(t, err)
	assert.Contains(t, string(df), "FROM alpine:latest")
}
