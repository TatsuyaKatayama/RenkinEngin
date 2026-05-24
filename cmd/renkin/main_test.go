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
