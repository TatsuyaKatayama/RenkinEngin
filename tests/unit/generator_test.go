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

func TestDockerfileGenerationMCPServerGitPreset(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/mcp-server-git.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		Docker:   config.DockerConf{BaseImage: "ubuntu:24.04"},
		LLM:      &config.LLMConf{Cmd: "codex"},
		ToolList: list,
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "apt-get install -y git")
	assert.Contains(t, dockerfile, "curl -LsSf https://astral.sh/uv/install.sh | sh")
	assert.Contains(t, dockerfile, "uv pip install --system --break-system-packages mcp-server-git")
	assert.Contains(t, dockerfile, "mcp-server-git")
	assert.Contains(t, dockerfile, "/root/.codex/config.toml")
	assert.NotContains(t, dockerfile, "/root/.gemini/settings.json")
	assert.Contains(t, dockerfile, "renkin-generate-llm-config")
	assert.Contains(t, dockerfile, `args = ["--repository", "/workspace"]`)
}

func TestRuntimeConfigGenerationForMCPToolWithoutLLM(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/mcp-server-git.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		Docker:   config.DockerConf{BaseImage: "ubuntu:24.04"},
		ToolList: list,
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "renkin-generate-llm-config")
	assert.Contains(t, dockerfile, "/root/.codex/config.toml")
	assert.Contains(t, dockerfile, "/root/.gemini/settings.json")
}

func TestDockerfileGenerationForgejoMCPPreset(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/forgejo-mcp.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		Docker:   config.DockerConf{BaseImage: "ubuntu:24.04"},
		LLM:      &config.LLMConf{Cmd: "codex"},
		ToolList: list,
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "apt-get install -y git")
	assert.Contains(t, dockerfile, "git clone --depth 1 https://github.com/goern/forgejo-mcp.git")
	assert.Contains(t, dockerfile, "go build -o /usr/local/bin/forgejo-mcp .")
	assert.Contains(t, dockerfile, "/root/.codex/config.toml")
	assert.NotContains(t, dockerfile, "/root/.gemini/settings.json")
	assert.Contains(t, dockerfile, "renkin-generate-llm-config")
	assert.Contains(t, dockerfile, "${FORGEJO_URL:-https://codeberg.org}")
}

func TestRuntimeConfigGenerationGemini(t *testing.T) {
	list, err := config.LoadToolList("../../presets/tools/masatools.toml")
	assert.NoError(t, err)
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		Docker:   config.DockerConf{BaseImage: "ubuntu:24.04"},
		LLM:      &config.LLMConf{Cmd: "gemini"},
		ToolList: list,
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "renkin-generate-llm-config")
	assert.NotContains(t, dockerfile, "/root/.codex/config.toml")
	assert.Contains(t, dockerfile, "/root/.gemini/settings.json")
	assert.Contains(t, dockerfile, "[mcp_servers.masatools]")
	assert.Contains(t, dockerfile, `"masatools": {`)
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

func TestEnvGenerationToolEnvironment(t *testing.T) {
	cfg := config.Config{
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{Name: "git", Type: "shell", Environment: []string{"GIT_USER_NAME", "GIT_USER_EMAIL"}},
			},
		},
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "GIT_USER_NAME=")
	assert.Contains(t, env, "GIT_USER_EMAIL=")
}

func TestDockerComposeGenerationToolEnvironment(t *testing.T) {
	cfg := config.Config{
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{Name: "git", Type: "shell", Environment: []string{"GIT_USER_NAME", "GIT_USER_EMAIL"}},
			},
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "environment:")
	assert.Contains(t, compose, "- GIT_USER_NAME")
	assert.Contains(t, compose, "- GIT_USER_EMAIL")
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

func TestEnvGenerationGemini(t *testing.T) {
	cfg := config.Config{
		LLM: &config.LLMConf{
			Cmd:      "gemini",
			AuthMode: "api_key",
		},
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "GEMINI_API_KEY=")
}

func TestDockerComposeGenerationGeminiBrowser(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{},
		LLM: &config.LLMConf{
			Cmd:      "gemini",
			AuthMode: "browser",
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "/root/.config/gemini")
}

func TestDockerComposeGenerationMCPEnvironment(t *testing.T) {
	cfg := config.Config{
		ToolList: config.ToolList{
			Tools: []config.Tool{
				{
					Name:        "mcp-tool",
					Type:        "mcp",
					Image:       "mcp-image",
					Port:        1234,
					Environment: []string{"MCP_VAR1"},
				},
			},
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "mcp-tool:")
	assert.Contains(t, compose, "environment:")
	assert.Contains(t, compose, "- MCP_VAR1")
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
