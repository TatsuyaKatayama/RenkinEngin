package generator

import (
	"bytes"
	"text/template"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
)

const dockerfileTemplate = `FROM {{.Docker.BaseImage}}

{{if .LLM}}
# LLM installation
{{.LLM.Install}}
{{end}}

# Shell tools installation
{{range .ToolList.Tools}}{{if eq .Type "shell"}}{{.Install}}
{{end}}{{end}}
WORKDIR /workspace
`

const dockerComposeTemplate = `services:
  llm-agent:
    build:
      context: .
{{- if .ProxyKeys}}
      args:
{{- range .ProxyKeys}}
        - {{.}}
{{- end}}
{{- end}}
    stdin_open: true
    tty: true
    extra_hosts:
      - "host.docker.internal:host-gateway"
    env_file: .env
{{- if or .DefaultEnv .EnvKeys}}
    environment:
{{- range .DefaultEnv}}
      - {{.}}
{{- end}}
{{- range .EnvKeys}}
      - {{.}}
{{- end}}
{{- end}}
{{if .LLM}}{{if .LLM.Ports}}
    ports:
{{- range .LLM.Ports}}
      - "{{.}}"
{{- end}}
{{end}}{{end}}
{{if or .Docker.Mounts (and .LLM (eq .LLM.AuthMode "browser"))}}
    volumes:
{{- range .Docker.Mounts}}
      - {{.Host}}:{{.Container}}
{{- end}}
{{if .LLM}}{{if eq .LLM.AuthMode "browser"}}
{{- range (index .ExtraMounts "llm-auth")}}
      - {{.Host}}:{{.Container}}
{{- end}}
{{- end}}{{- end}}
{{- end}}

{{- range .ToolList.Tools}}{{if eq .Type "mcp"}}
  {{.Name}}:
    image: {{.Image}}
    ports:
      - "{{.Port}}:{{.Port}}"
{{- if .Environment}}
    environment:
{{- range .Environment}}
      - {{.}}
{{- end}}
{{- end}}
{{- end}}{{end}}
`

const envTemplate = `{{range .EnvKeys}}{{.}}=
{{end}}`

type GeneratorData struct {
	config.Config
	EnvKeys     []string
	DefaultEnv  []string
	ProxyKeys   []string
	ExtraMounts map[string][]config.Mount
}

func GenerateDockerfile(cfg config.Config) (string, error) {
	tmpl, err := template.New("Dockerfile").Parse(dockerfileTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GenerateDockerCompose(cfg config.Config) (string, error) {
	tmpl, err := template.New("docker-compose.yml").Parse(dockerComposeTemplate)
	if err != nil {
		return "", err
	}

	data := GeneratorData{
		Config:      cfg,
		EnvKeys:     cfg.CollectEnvKeys(),
		DefaultEnv:  []string{},
		ProxyKeys:   config.GetActiveProxyKeys(),
		ExtraMounts: make(map[string][]config.Mount),
	}

	if cfg.LLM != nil {
		llmType, _ := cfg.LLM.GetType()
		if llmType == "gemini" {
			data.DefaultEnv = append(data.DefaultEnv, "GEMINI_TRUST_WORKSPACE=true")
		} else if llmType == "codex" {
			data.DefaultEnv = append(data.DefaultEnv, "CODEX_TRUST_WORKSPACE=true")
		}
	}

	if cfg.LLM != nil && cfg.LLM.AuthMode == "browser" {
		llmType, _ := cfg.LLM.GetType()
		home, _ := config.GetHomeDir()
		switch llmType {
		case "claude":
			data.ExtraMounts["llm-auth"] = []config.Mount{{Host: home + "/.claude", Container: "/root/.claude"}}
		case "gemini":
			data.ExtraMounts["llm-auth"] = []config.Mount{{Host: home + "/.config/gemini", Container: "/root/.config/gemini"}}
		case "codex":
			data.ExtraMounts["llm-auth"] = []config.Mount{{Host: home + "/.codex", Container: "/root/.codex"}}
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GenerateEnv(cfg config.Config) (string, error) {
	tmpl, err := template.New(".env").Parse(envTemplate)
	if err != nil {
		return "", err
	}

	envKeys := cfg.CollectEnvKeys()
	if len(envKeys) == 0 {
		return "", nil
	}

	data := struct {
		EnvKeys []string
	}{
		EnvKeys: envKeys,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
