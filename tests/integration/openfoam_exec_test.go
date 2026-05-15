package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDockerExecIcoFoamHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-of-test")
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Use a mock install for faster testing, but name it like openfoam
	dockerConf := `base_image = "ubuntu:24.04"
[[mount]]
host = "./workspace"
container = "/workspace"
`
	// Mock icoFoam by creating a script that outputs help
	toolList := `[[tool]]
name = "mock-openfoam"
type = "shell"
install = """
RUN echo '#!/bin/bash\necho "icoFoam help text"' > /usr/local/bin/icoFoam && \
    chmod +x /usr/local/bin/icoFoam
"""
`

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(toolList), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	// Build and Start
	// We need to be in the target directory
	startCmd := exec.Command(binPath, "start")
	startCmd.Dir = targetDir
	// Note: 'renkin start' in current implementation will try to attach if metadata exists.
	// Since we didn't provide --llm, it won't attach.
	if out, err := startCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin start failed: %v\n%s", err, string(out))
	}
	
	// Wait a bit for container to be ready
	time.Sleep(2 * time.Second)

	// Now test docker exec via renkin (or direct docker exec to verify the environment)
	// The user asked for "docker exec it でicoFoam --help"
	// We can use 'docker exec' directly to verify the container 'llm-agent' (default name in compose)
	execCmd := exec.Command("docker", "exec", "target-llm-agent-1", "icoFoam", "--help")
	// Note: docker compose usually prefixes with directory name. 
	// In our generator, service name is 'llm-agent'.
	// Let's check the container name or use 'docker compose exec'
	execCmd = exec.Command("docker", "compose", "exec", "-T", "llm-agent", "icoFoam", "--help")
	execCmd.Dir = targetDir
	
	output, err := execCmd.CombinedOutput()
	assert.NoError(t, err, string(output))
	assert.Contains(t, string(output), "icoFoam help text")

	// Cleanup
	endCmd := exec.Command(binPath, "end")
	endCmd.Dir = targetDir
	endCmd.Run()
}
