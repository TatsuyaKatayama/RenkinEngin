package unit

import (
	"os"
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestProxySupport(t *testing.T) {
	// Set mock proxy env vars
	os.Setenv("HTTP_PROXY", "http://proxy.test:8080")
	os.Setenv("no_proxy", "localhost")
	defer os.Unsetenv("HTTP_PROXY")
	defer os.Unsetenv("no_proxy")

	cfg := config.Config{
		Docker: config.DockerConf{},
	}

	// Test docker-compose.yml contains build args for proxy
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "args:")
	assert.Contains(t, compose, "- HTTP_PROXY")
	assert.Contains(t, compose, "- no_proxy")

	// Test .env contains proxy keys
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "HTTP_PROXY=")
	assert.Contains(t, env, "no_proxy=")
}
