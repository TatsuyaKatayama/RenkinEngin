package main

import (
	"testing"

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
