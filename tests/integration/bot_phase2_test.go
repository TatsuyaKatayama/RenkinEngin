package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBotRunOnceDispatchesDetectedDiscordMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var toolCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			toolCalls++
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			require.NoError(t, json.Unmarshal(req.Params, &params))
			assert.Equal(t, "read_messages", params.Name)
			assert.Equal(t, "c1", params.Arguments["channelId"])
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"messages":[{"id":"101","channel_id":"c1","content":"work","author":{"id":"u1","bot":false}}]}}`))
		default:
			t.Fatalf("unexpected MCP method %s", req.Method)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	require.NoError(t, buildCmd.Run())

	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".env"), []byte("DISCORD_MCP_URL="+server.URL+"\nDISCORD_CHANNEL_ID=c1\nDISCORD_BOT_USER_ID=bot1\n"), 0644))
	runCmd := exec.Command(
		binPath,
		"bot",
		"run-once",
		"--cmd", `true`,
	)
	runCmd.Dir = targetDir
	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, string(output))

	assert.Equal(t, 1, toolCalls)

	state, err := os.ReadFile(filepath.Join(targetDir, ".renkin_bot_state.json"))
	require.NoError(t, err)
	assert.Contains(t, string(state), `"last_message_id": "101"`)
	assert.Contains(t, string(state), `"board_item_id": "101"`)
	assert.Contains(t, string(state), `"state": "in_flight"`)
}

func TestBotStartStatusStopLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"messages":[]}}`))
		default:
			t.Fatalf("unexpected MCP method %s", req.Method)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	require.NoError(t, buildCmd.Run())

	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".env"), []byte("DISCORD_MCP_URL="+server.URL+"\nDISCORD_CHANNEL_ID=c1\n"), 0644))
	pidFile := filepath.Join(targetDir, "bot.pid")
	logFile := filepath.Join(targetDir, "bot.log")
	stateFile := filepath.Join(targetDir, "bot-state.json")

	startCmd := exec.Command(
		binPath,
		"bot",
		"start",
		"--cmd", "true",
		"--interval", "100ms",
		"--pid-file", pidFile,
		"--log-file", logFile,
		"--state", stateFile,
	)
	startCmd.Dir = targetDir
	startOut, err := startCmd.CombinedOutput()
	require.NoError(t, err, string(startOut))
	assert.Contains(t, string(startOut), "Bot started:")

	t.Cleanup(func() {
		stopCmd := exec.Command(binPath, "bot", "stop", "--pid-file", pidFile)
		stopCmd.Dir = targetDir
		_ = stopCmd.Run()
	})

	require.Eventually(t, func() bool {
		statusCmd := exec.Command(binPath, "bot", "status", "--pid-file", pidFile, "--state", stateFile)
		statusCmd.Dir = targetDir
		out, err := statusCmd.CombinedOutput()
		return err == nil && strings.Contains(string(out), "Bot: running")
	}, 2*time.Second, 100*time.Millisecond)

	stopCmd := exec.Command(binPath, "bot", "stop", "--pid-file", pidFile)
	stopCmd.Dir = targetDir
	stopOut, err := stopCmd.CombinedOutput()
	require.NoError(t, err, string(stopOut))
	assert.Contains(t, string(stopOut), "Bot stopped:")

	statusCmd := exec.Command(binPath, "bot", "status", "--pid-file", pidFile, "--state", stateFile)
	statusCmd.Dir = targetDir
	statusOut, err := statusCmd.CombinedOutput()
	require.NoError(t, err, string(statusOut))
	assert.Contains(t, string(statusOut), "Bot: stopped")
}
