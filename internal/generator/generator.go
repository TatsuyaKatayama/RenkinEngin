package generator

import (
	"bytes"
	"os"
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
    env_file: .env
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
{{- end}}{{end}}
`

const envTemplate = `{{range .EnvKeys}}{{.}}=
{{end}}`

type GeneratorData struct {
	config.Config
	EnvKeys     []string
	ProxyKeys   []string
	ExtraMounts map[string][]config.Mount
}

var proxyEnvNames = []string{
	"HTTP_PROXY", "http_proxy",
	"HTTPS_PROXY", "https_proxy",
	"NO_PROXY", "no_proxy",
}

func getActiveProxyKeys() []string {
	var keys []string
	for _, name := range proxyEnvNames {
		if os.Getenv(name) != "" {
			keys = append(keys, name)
		}
	}
	return keys
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
		ProxyKeys:   getActiveProxyKeys(),
		ExtraMounts: make(map[string][]config.Mount),
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

	var envKeys []string
	if cfg.LLM != nil && cfg.LLM.AuthMode != "browser" {
		envKeys = append(envKeys, cfg.LLM.GetEnvKeys()...)
	}
	// Add proxy keys to .env as well
	envKeys = append(envKeys, getActiveProxyKeys()...)

	// Add tool environment keys to .env
	for _, t := range cfg.ToolList.Tools {
		envKeys = append(envKeys, t.Environment...)
	}

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
