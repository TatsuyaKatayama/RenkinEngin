package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/bot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStartupBoardAdapter struct {
	gotCheckpoint bot.Checkpoint
	items         []bot.BoardItem
	next          bot.Checkpoint
}

func (f *fakeStartupBoardAdapter) PollSince(_ context.Context, cp bot.Checkpoint) ([]bot.BoardItem, bot.Checkpoint, error) {
	f.gotCheckpoint = cp
	return f.items, f.next, nil
}

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
		"DISCORD_BOT_USERNAME":    "bot-name",
		"RENKIN_BOT_DISPATCH_CMD": "renkin start --cmd true",
		"RENKIN_BOT_WEBHOOK_URL":  "http://webhook.example",
	}
	opts, err := resolveBotOptions("discord", "", "", "", "", "", "", time.Second, 2, 3*time.Second, 4*time.Second, "", true, func(key string) string {
		return env[key]
	})

	assert.NoError(t, err)
	assert.Equal(t, "discord", opts.Board)
	assert.Equal(t, "http://mcp.example/mcp", opts.MCPURL)
	assert.Equal(t, "channel-1", opts.ChannelID)
	assert.Equal(t, "bot-1", opts.BotUserID)
	assert.Equal(t, "bot-name", opts.BotUsername)
	assert.Equal(t, "renkin start --cmd true", opts.DispatchCommand)
	assert.Equal(t, ".renkin_bot_state.json", opts.StatePath)
	assert.Equal(t, time.Second, opts.Interval)
	assert.Equal(t, 2, opts.MaxRetries)
	assert.Equal(t, 3*time.Second, opts.RestartDelay)
	assert.Equal(t, 4*time.Second, opts.Deadline)
	assert.Equal(t, "http://webhook.example", opts.WebhookURL)
	assert.True(t, opts.CheckpointOnStart)
}

func TestResolveBotOptionsRequiresChannelAndCommand(t *testing.T) {
	_, err := resolveBotOptions("discord", "", "", "", "", "", "", time.Second, 1, 0, time.Second, "", false, func(string) string {
		return ""
	})

	assert.ErrorContains(t, err, "bot channel ID is required")

	_, err = resolveBotOptions("discord", "", "channel-1", "", "", "", "", time.Second, 1, 0, time.Second, "", false, func(string) string {
		return ""
	})

	assert.ErrorContains(t, err, "bot dispatch command is required")
}

func TestResolveBotOptionsRejectsUnsupportedBoard(t *testing.T) {
	_, err := resolveBotOptions("masabbs", "", "", "", "", "", "", time.Second, 1, 0, time.Second, "", false, func(string) string {
		return ""
	})

	assert.ErrorContains(t, err, `unsupported bot board "masabbs"`)
}

func TestResolveBotOptionsDefaultsDispatchCommandFromBotLoop(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".renkin", "conf"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".renkin", "conf", "bot-loop.sh"), []byte("#!/bin/bash\n"), 0755))

	opts, err := resolveBotOptions("discord", "", "channel-1", "", "", "", "", time.Second, 1, 0, time.Second, "", false, func(string) string {
		return ""
	})

	require.NoError(t, err)
	assert.Equal(t, "docker compose exec -T -e RENKIN_BOARD_ITEM_ID -e RENKIN_BOARD_CHANNEL_ID -e RENKIN_BOARD_AUTHOR_ID -e RENKIN_BOARD_CONTENT -e RENKIN_BOARD_CREATED_AT llm-agent bash -lc 'renkin-generate-llm-config; bash /renkin-conf/bot-loop.sh'", opts.DispatchCommand)
}

func TestBotRunArgs(t *testing.T) {
	args := botRunArgs(botOptions{
		Board:             "discord",
		MCPURL:            "http://localhost:8085/mcp",
		ChannelID:         "c1",
		BotUserID:         "bot1",
		BotUsername:       "bot-name",
		DispatchCommand:   "renkin start --cmd true",
		StatePath:         "state.json",
		Interval:          2 * time.Second,
		MaxRetries:        3,
		RestartDelay:      4 * time.Second,
		Deadline:          5 * time.Second,
		WebhookURL:        "http://webhook.example",
		CheckpointOnStart: true,
	})

	assert.Equal(t, []string{
		"bot", "run",
		"--board", "discord",
		"--mcp-url", "http://localhost:8085/mcp",
		"--channel-id", "c1",
		"--bot-user-id", "bot1",
		"--bot-username", "bot-name",
		"--cmd", "renkin start --cmd true",
		"--state", "state.json",
		"--interval", "2s",
		"--max-retries", "3",
		"--restart-delay", "4s",
		"--deadline", "5s",
		"--webhook-url", "http://webhook.example",
		"--checkpoint-on-start", "true",
	}, args)
}

func TestRecordStartupCheckpointSkipsExistingItemsWithoutDispatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), bot.DefaultStateFileName)
	store := bot.NewStateStore(path)
	require.NoError(t, store.Save(bot.State{Checkpoint: bot.Checkpoint{LastMessageID: "100"}}))
	adapter := &fakeStartupBoardAdapter{
		items: []bot.BoardItem{
			{ID: "101", ChannelID: "c1", AuthorID: "u1"},
			{ID: "105", ChannelID: "c1", AuthorID: "u2"},
		},
		next: bot.Checkpoint{LastMessageID: "105"},
	}

	err := recordStartupCheckpoint(context.Background(), adapter, store)

	require.NoError(t, err)
	assert.Equal(t, bot.Checkpoint{LastMessageID: "100"}, adapter.gotCheckpoint)
	state, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "105", state.Checkpoint.LastMessageID)
	assert.False(t, state.Checkpoint.LastCheckedAt.IsZero())
	assert.Nil(t, state.Dispatch)
}

func TestDefaultBotPaths(t *testing.T) {
	assert.Equal(t, ".renkin/bot.pid", defaultBotPIDPath(""))
	assert.Equal(t, "custom.pid", defaultBotPIDPath("custom.pid"))
	assert.Equal(t, ".renkin/logs/bot.log", defaultBotLogPath(""))
	assert.Equal(t, "custom.log", defaultBotLogPath("custom.log"))
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
