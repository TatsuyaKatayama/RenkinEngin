package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestDockerExecMultiTool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-multi-test")
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	dockerConf := `base_image = "ubuntu:24.04"`
	
	// OpenFOAM + Python
	toolList := `
[[tool]]
preset = "openfoam2512"
[[tool]]
preset = "python-post"
`
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	
	os.WriteFile(filepath.Join(targetDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(targetDir, "tool_list.toml"), []byte(toolList), 0644)

	// Ensure presets exist
	presetsDir := filepath.Join(tmpDir, "presets", "tools")
	os.MkdirAll(presetsDir, 0755)
	if err := utils.CopyFile("../../presets/tools/openfoam2512.toml", filepath.Join(presetsDir, "openfoam2512.toml")); err != nil {
		t.Fatalf("failed to copy openfoam preset: %v", err)
	}
	if err := utils.CopyFile("../../presets/tools/python-post.toml", filepath.Join(presetsDir, "python-post.toml")); err != nil {
		t.Fatalf("failed to copy python-post preset: %v", err)
	}

	// assign
	assignCmd := exec.Command(binPath, "assign", targetDir)
	assignCmd.Dir = tmpDir
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	// build & up
	// Need to use -f with full path to compose file
	composeFile := filepath.Join(targetDir, "docker-compose.yml")
	upCmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "--build")
	if out, err := upCmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose up failed: %v\n%s", err, string(out))
	}
	
	// Verify Foam
	execFoam := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", "llm-agent", "bash", "-c", "source /usr/lib/openfoam/openfoam2512/etc/bashrc && icoFoam -help")
	outFoam, err := execFoam.CombinedOutput()
	assert.NoError(t, err, string(outFoam))
	assert.Contains(t, string(outFoam), "Usage")

	// Verify Python
	execPy := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", "llm-agent", "python3", "-c", "import foamlib, DyMat; print('success')")
	outPy, err := execPy.CombinedOutput()
	assert.NoError(t, err, string(outPy))
	assert.Contains(t, string(outPy), "success")

	// Cleanup
	downCmd := exec.Command(binPath, "kaiko", "--yes")
	downCmd.Dir = targetDir
	downCmd.Run()
}
