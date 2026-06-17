package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoopE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "renkin-loop-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Create a dynamic directory preset for fake-llm to test directory-based presets without dirtying the repository.
	fakePresetDir := filepath.Join(tmpDir, "presets", "llms", "fake-llm")
	if err := os.MkdirAll(fakePresetDir, 0755); err != nil {
		t.Fatalf("failed to create fake preset dir: %v", err)
	}

	llmTOML := `cmd = "fake-llm"
loop_cmd = 'fake-llm --session-id {session_id} -r latest --prompt "$(cat /renkin-conf/bot_prompt.md)"'
restart_policy = "on-failure"
log_dir = ".renkin/logs"
stdout_log = "stdout.log"
stderr_log = "stderr.log"
install = """
RUN apt-get update && apt-get install -y curl && \
    echo '#!/bin/bash\\necho "fake-llm executed with args: $*"\\necho "Current session_id: $2"\\nif [[ "$*" == *"fail"* ]]; then\\n  exit 1\\nfi\\nexit 0' > /usr/local/bin/fake-llm && \
    chmod +x /usr/local/bin/fake-llm
"""
`
	botLoopScript := `#!/bin/bash
# True Renkin Loop Command Runner (fake-llm)
AGENT_ID=${AGENT_ID:-default-agent}
echo "Starting True Renkin Loop Iteration for ${AGENT_ID}..."
{llm_cmd}
`

	botPromptMarkdown := `## Task
Analyze the files in your workspace. Check if there are any new files or changes, write a brief summary report of the workspace status to stdout, and print the current time.
`

	os.WriteFile(filepath.Join(fakePresetDir, "llm.toml"), []byte(llmTOML), 0644)
	os.WriteFile(filepath.Join(fakePresetDir, "bot-loop.sh"), []byte(botLoopScript), 0755)
	os.WriteFile(filepath.Join(fakePresetDir, "bot_prompt.md"), []byte(botPromptMarkdown), 0644)

	dockerConf := `base_image = "ubuntu:24.04"
[[mount]]
host = "./workspace"
container = "/workspace"
`

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte("[[tool]]\nname=\"test\"\ntype=\"shell\"\ninstall=\"RUN echo install\""), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--llm", "fake-llm",
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
	)
	assignCmd.Dir = tmpDir
	if out, err := assignCmd.CombinedOutput(); err != nil {
		t.Fatalf("renkin assign failed: %v\n%s", err, string(out))
	}

	// Verify that bot_prompt.md was automatically generated
	botPromptPath := filepath.Join(targetDir, ".renkin", "conf", "bot_prompt.md")
	assert.FileExists(t, botPromptPath)

	// Print .renkin_metadata.toml for debugging
	metaBytes, err := os.ReadFile(filepath.Join(targetDir, ".renkin_metadata.toml"))
	if err == nil {
		t.Logf("Generated .renkin_metadata.toml:\n%s", string(metaBytes))
	}

	// Build the Docker Compose environment
	buildCmd2 := exec.Command("docker", "compose", "build")
	buildCmd2.Dir = targetDir
	if out, err := buildCmd2.CombinedOutput(); err != nil {
		t.Fatalf("docker compose build failed: %v\n%s", err, string(out))
	}

	// 1. Test Default Loop (without opt value)
	// We run it in a background process because it's a looping command on the host.
	// But since its restart policy is "on-failure" and it exits successfully (exit 0),
	// it should run exactly once and exit successfully! So we can wait for it.
	startCmd := exec.Command(binPath, "start", "--loop")
	startCmd.Dir = targetDir
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	err = startCmd.Run()
	assert.NoError(t, err)

	// Verify stdout.log and session markers
	stdoutLogPath := filepath.Join(targetDir, ".renkin", "logs", "stdout.log")
	assert.FileExists(t, stdoutLogPath)

	logContent, err := os.ReadFile(stdoutLogPath)
	if err != nil {
		t.Fatalf("failed to read stdout log: %v", err)
	}

	assert.Contains(t, string(logContent), "=== SESSION START:")
	assert.Contains(t, string(logContent), "fake-llm executed with args:")
	assert.Contains(t, string(logContent), "=== SESSION END:")

	// 2. Test Custom Command Loop (non-zero exit and on-failure restart policy)
	// We run 'fake-llm fail' in loop mode. Since the policy is "on-failure", it should try to restart.
	// We launch it, wait 3 seconds, and then kill it to verify that it loops.
	loopFailCmd := exec.Command(binPath, "start", "--loop=fake-llm fail")
	loopFailCmd.Dir = targetDir

	if err := loopFailCmd.Start(); err != nil {
		t.Fatalf("failed to start loopFailCmd: %v", err)
	}

	// Let it run for 3 seconds so it triggers at least one restart
	time.Sleep(3 * time.Second)
	loopFailCmd.Process.Kill()

	// Read logs and verify restarts are logged
	logContentFail, _ := os.ReadFile(stdoutLogPath)
	assert.Contains(t, string(logContentFail), "Command: fake-llm fail")
	assert.Contains(t, string(logContentFail), "Exit Code: 1")
	assert.True(t, strings.Count(string(logContentFail), "=== SESSION START:") >= 2)

	// Clean up containers
	downCmd := exec.Command(binPath, "kaiko", "--yes")
	downCmd.Dir = targetDir
	downCmd.Run()
}
