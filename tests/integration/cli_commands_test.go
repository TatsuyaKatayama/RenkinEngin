package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLICommandLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "renkin-cli-lifecycle-test")
	defer os.RemoveAll(tmpDir)
	t.Setenv("GEMINI_API_KEY", "dummy-key")

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build renkin: %v\n%s", err, string(out))
	}

	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)

	// 1. Assign
	assignCmd := exec.Command(binPath, "assign", targetDir, "--llm", "gemini")
	assignCmd.Dir = "../../"
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	// 2. Start (using --cmd true to exit immediately)
	startCmd := exec.Command(binPath, "start", "--cmd", "true")
	startCmd.Dir = targetDir
	if out, err := startCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin start failed: %v\n%s", err, string(out))
	}

	// Verify container is created (it might be exited but exists)
	psCmd := exec.Command("docker", "compose", "ps", "-a", "--format", "json")
	psCmd.Dir = targetDir
	out, _ := psCmd.Output()
	assert.Contains(t, string(out), "llm-agent")

	// 3. Stop
	stopCmd := exec.Command(binPath, "stop")
	stopCmd.Dir = targetDir
	if out, err := stopCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin stop failed: %v\n%s", err, string(out))
	}

	// Verify container is gone
	psCmd = exec.Command("docker", "compose", "ps", "-a", "--format", "json")
	psCmd.Dir = targetDir
	out, _ = psCmd.Output()
	assert.NotContains(t, string(out), "llm-agent")

	// 4. Restart
	restartCmd := exec.Command(binPath, "restart", "--cmd", "true")
	restartCmd.Dir = targetDir
	if out, err := restartCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin restart failed: %v\n%s", err, string(out))
	}
	// Check again with -a to see if the container exists (it might have exited since --cmd true)
	psCmd = exec.Command("docker", "compose", "ps", "-a", "--format", "json")
	psCmd.Dir = targetDir
	out, _ = psCmd.Output()
	assert.Contains(t, string(out), "llm-agent")

	// 5. Kaiko (need to provide 'y' to confirmation)
	kaikoCmd := exec.Command(binPath, "kaiko")
	kaikoCmd.Dir = targetDir
	stdin, _ := kaikoCmd.StdinPipe()
	go func() {
		defer stdin.Close()
		stdin.Write([]byte("y\n"))
	}()
	if out, err := kaikoCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin kaiko failed: %v\n%s", err, string(out))
	}

	// Verify image is also gone (optional check, might be shared)
	// For now, check container is gone
	psCmd = exec.Command("docker", "compose", "ps", "-a", "--format", "json")
	psCmd.Dir = targetDir
	out, _ = psCmd.Output()
	assert.NotContains(t, string(out), "llm-agent")
}
