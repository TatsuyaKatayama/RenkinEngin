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
			Cmd:     "gemini",
			Install: "RUN echo install-llm",
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
	assert.Contains(t, dockerfile, "RUN echo install-llm")
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
	assert.Contains(t, dockerfile, "/root/.codex")
	assert.Contains(t, dockerfile, "/root/.gemini")
	assert.Contains(t, dockerfile, "renkin-generate-llm-config")
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
	// Startup scripts for tools are still generated
	assert.Contains(t, dockerfile, "mcp-server-git")
}

func TestRuntimeConfigGenerationGemini(t *testing.T) {
	tpData, err := config.LoadToolPreset("../../presets/tools", "masatools")
	assert.NoError(t, err)
	list := tpData.ToolList
	err = list.ResolvePresets("../../presets/tools")
	assert.NoError(t, err)

	cfg := config.Config{
		Docker: config.DockerConf{BaseImage: "ubuntu:24.04"},
		LLM: &config.LLMConf{
			Cmd: "gemini",
			RuntimeConfigs: []config.RuntimeConfig{
				{Source: "settings.json", Target: "/root/.gemini/settings.json"},
			},
			SkillFile: "GEMINI.md",
		},
		ToolList: list,
	}
	dockerfile, err := generator.GenerateDockerfile(cfg)
	assert.NoError(t, err)
	assert.Contains(t, dockerfile, "renkin-generate-llm-config")

	// Test new config resolution logic from /renkin-conf
	assert.Contains(t, dockerfile, "if [ -f /renkin-conf/settings.json ]; then")
	assert.Contains(t, dockerfile, "python3 -c 'import os, sys; print(os.path.expandvars(sys.stdin.read()))' < /renkin-conf/settings.json > /root/.gemini/settings.json")

	// Test SkillFile placement
	assert.Contains(t, dockerfile, "if [ -f /renkin-conf/GEMINI.md ]; then")
	assert.Contains(t, dockerfile, "cp /renkin-conf/GEMINI.md /root/.gemini/GEMINI.md")
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

func TestDockerComposeGenerationHomeMounts(t *testing.T) {
	cfg := config.Config{
		Docker: config.DockerConf{},
		LLM: &config.LLMConf{
			Cmd: "agy",
			HomeMounts: []config.HomeMount{
				{HostDir: ".renkin/gemini", ContainerDir: "/root/.gemini"},
			},
		},
	}
	compose, err := generator.GenerateDockerCompose(cfg)
	assert.NoError(t, err)
	assert.Contains(t, compose, "- ./.renkin/gemini:/root/.gemini")
	assert.Contains(t, compose, "- ./.renkin/conf:/renkin-conf:ro")
}

func TestEnvGenerationCodex(t *testing.T) {
	cfg := config.Config{
		LLM: &config.LLMConf{
			Cmd: "codex",
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

func TestEnvGenerationGemini(t *testing.T) {
	cfg := config.Config{
		LLM: &config.LLMConf{
			Cmd: "gemini",
		},
	}
	env, err := generator.GenerateEnv(cfg)
	assert.NoError(t, err)
	assert.Contains(t, env, "GEMINI_API_KEY=")
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
