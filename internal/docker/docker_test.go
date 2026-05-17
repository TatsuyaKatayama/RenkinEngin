package docker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComposeEnvMergesOnlyNonEmptyEnvFileValues(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, []byte("GIT_USER_NAME=EnvFileUser\nGIT_USER_EMAIL=\nNEW_VALUE=from-env\n"), 0644)
	assert.NoError(t, err)

	env := composeEnv([]string{"GIT_USER_NAME=HostUser", "GIT_USER_EMAIL=host@example.com"}, envPath)

	assert.Contains(t, env, "GIT_USER_NAME=EnvFileUser")
	assert.Contains(t, env, "GIT_USER_EMAIL=host@example.com")
	assert.Contains(t, env, "NEW_VALUE=from-env")
}
