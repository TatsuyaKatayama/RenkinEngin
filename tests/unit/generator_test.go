package unit

import (
	"testing"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/stretchr/testify/assert"
)

func TestDockerfileGeneration(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{BaseImage: "ubuntu:24.04"},
		LLM: &config.LLMConf{
			Cmd:     "claude",
			Install: "RUN curl -fsSL https://claude.ai/install.sh | sh",
		},
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{Name: "openfoam", Type: "shell", Install: "RUN apt-get install -y openfoam2412"},
			},
		},
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "FROM ubuntu:24.04")
	assert.Contains(t, dockerfile, "RUN curl -fsSL https://claude.ai/install.sh | sh")
	assert.Contains(t, dockerfile, "RUN apt-get install -y openfoam2412")
	assert.Contains(t, dockerfile, "WORKDIR /workspace")
}

func TestDockerfileGenerationNoLLM(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{BaseImage: "ubuntu:24.04"},
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{Name: "python", Type: "shell", Install: "RUN apt-get install -y python3"},
			},
		},
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.NotContains(t, dockerfile, "LLM installation")
	assert.Contains(t, dockerfile, "RUN apt-get install -y python3")
}

func TestDockerfileGenerationNoTools(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{BaseImage: "ubuntu:24.04"},
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "FROM ubuntu:24.04")
	assert.Contains(t, dockerfile, "WORKDIR /workspace")
}

func TestDockerComposeGeneration(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{
			Mounts: []config.Mount{
				{Host: "./workspace", Container: "/workspace"},
			},
		},
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{Name: "lightrag", Type: "mcp", Image: "lightrag/server:latest", Port: 8080},
			},
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "llm-agent:")
	assert.Contains(t, compose, "lightrag:")
	assert.Contains(t, compose, "image: lightrag/server:latest")
	assert.Contains(t, compose, "- \"8080:8080\"")
	assert.Contains(t, compose, "- ./workspace:/workspace")
}

func TestDockerComposeGenerationNoMCP(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{
			Mounts: []config.Mount{{Host: "./w", Container: "/w"}},
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "llm-agent:")
	assert.NotContains(t, compose, "image:") // No MCP image
}

func TestDockerComposeGenerationBrowserAuth(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{},
		LLM: &config.LLMConf{
			Cmd:      "claude",
			AuthMode: "browser",
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "/root/.claude")
	assert.Contains(t, compose, "env_file: .env")

	// Check env generation for browser mode
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Empty(t, env)
}

func TestEnvGenerationCodex(t *testing.T) {
	cfg := config.Config{
		LLM: &config.LLMConf{
			Cmd:      "codex",
			AuthMode: "api_key",
		},
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "OPENAI_API_KEY=")
}

func TestDockerComposeGenerationCodexBrowser(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{},
		LLM: &config.LLMConf{
			Cmd:      "codex",
			AuthMode: "browser",
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "/root/.codex")
}

func TestEnvGenerationOpencode(t *testing.T) {
	cfg := config.Config{
		LLM: &config.LLMConf{
			Cmd:      "opencode",
			AuthMode: "api_key",
		},
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "ANTHROPIC_API_KEY=")
	assert.Contains(t, env, "OPENAI_API_KEY=")
	assert.Contains(t, env, "GEMINI_API_KEY=")
}
