package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDetermineCommand(t *testing.T) {
	tests := []struct {
		name        string
		metaLLMCmd  string
		overrideCmd string
		expected    string
	}{
		{
			name:        "both empty",
			metaLLMCmd:  "",
			overrideCmd: "",
			expected:    "",
		},
		{
			name:        "only meta cmd",
			metaLLMCmd:  "claude",
			overrideCmd: "",
			expected:    "claude",
		},
		{
			name:        "only override cmd",
			metaLLMCmd:  "",
			overrideCmd: "bash",
			expected:    "bash",
		},
		{
			name:        "override takes precedence",
			metaLLMCmd:  "claude",
			overrideCmd: "bash",
			expected:    "bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := determineCommand(tt.metaLLMCmd, tt.overrideCmd)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestMissingEnvKeysAcceptsGeneratedEnvFileValues(t *testing.T) {
	missing := missingEnvKeys(
		[]string{"AGENT_ID", "OPENAI_API_KEY", "NATS_URL"},
		func(key string) string {
			if key == "OPENAI_API_KEY" {
				return "from-host"
			}
			return ""
		},
		map[string]string{
			"AGENT_ID": "agent-from-env-file",
		},
	)

	assert.Equal(t, []string{"NATS_URL"}, missing)
}

func TestResolveBotOptions(t *testing.T) {
	env := map[string]string{
		"DISCORD_MCP_URL":         "http://mcp.example/mcp",
		"DISCORD_CHANNEL_ID":      "channel-1",
		"DISCORD_BOT_USER_ID":     "bot-1",
		"RENKIN_BOT_DISPATCH_CMD": "renkin start --cmd true",
	}
	opts, err := resolveBotOptions("", "", "", "", "", time.Second, 2, 3*time.Second, 4*time.Second, func(key string) string {
		return env[key]
	})

	assert.NoError(t, err)
	assert.Equal(t, "http://mcp.example/mcp", opts.MCPURL)
	assert.Equal(t, "channel-1", opts.ChannelID)
	assert.Equal(t, "bot-1", opts.BotUserID)
	assert.Equal(t, "renkin start --cmd true", opts.DispatchCommand)
	assert.Equal(t, ".renkin_bot_state.json", opts.StatePath)
	assert.Equal(t, time.Second, opts.Interval)
	assert.Equal(t, 2, opts.MaxRetries)
	assert.Equal(t, 3*time.Second, opts.RestartDelay)
	assert.Equal(t, 4*time.Second, opts.Deadline)
}

func TestResolveBotOptionsRequiresChannelAndCommand(t *testing.T) {
	_, err := resolveBotOptions("", "", "", "", "", time.Second, 1, 0, time.Second, func(string) string {
		return ""
	})

	assert.ErrorContains(t, err, "bot channel ID is required")

	_, err = resolveBotOptions("", "channel-1", "", "", "", time.Second, 1, 0, time.Second, func(string) string {
		return ""
	})

	assert.ErrorContains(t, err, "bot dispatch command is required")
}

func TestShouldRestartLoop(t *testing.T) {
	tests := []struct {
		name            string
		restartPolicy   string
		exitCode        int
		expectedRestart bool
		expectedKnown   bool
	}{
		{
			name:            "always restarts on success",
			restartPolicy:   "always",
			exitCode:        0,
			expectedRestart: true,
			expectedKnown:   true,
		},
		{
			name:            "always restarts on failure",
			restartPolicy:   "always",
			exitCode:        1,
			expectedRestart: true,
			expectedKnown:   true,
		},
		{
			name:            "on failure restarts on failure",
			restartPolicy:   "on-failure",
			exitCode:        1,
			expectedRestart: true,
			expectedKnown:   true,
		},
		{
			name:            "on failure stops on success",
			restartPolicy:   "on-failure",
			exitCode:        0,
			expectedRestart: false,
			expectedKnown:   true,
		},
		{
			name:            "on success restarts on success",
			restartPolicy:   "on-success",
			exitCode:        0,
			expectedRestart: true,
			expectedKnown:   true,
		},
		{
			name:            "on success stops on failure",
			restartPolicy:   "on-success",
			exitCode:        1,
			expectedRestart: false,
			expectedKnown:   true,
		},
		{
			name:            "never stops",
			restartPolicy:   "never",
			exitCode:        0,
			expectedRestart: false,
			expectedKnown:   true,
		},
		{
			name:            "empty policy stops",
			restartPolicy:   "",
			exitCode:        0,
			expectedRestart: false,
			expectedKnown:   true,
		},
		{
			name:            "unknown policy is not known",
			restartPolicy:   "sometimes",
			exitCode:        0,
			expectedRestart: false,
			expectedKnown:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restart, known := shouldRestartLoop(tt.restartPolicy, tt.exitCode)
			assert.Equal(t, tt.expectedRestart, restart)
			assert.Equal(t, tt.expectedKnown, known)
		})
	}
}
