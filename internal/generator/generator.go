package generator

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
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
{{.RuntimeConfigInstall}}

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
{{- if .HealthTools}}
    depends_on:
{{- range .HealthTools}}
      {{.Name}}:
        condition: service_healthy
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
{{if or .Docker.Mounts (and .LLM .LLM.HomeMounts)}}
    volumes:
{{- range .Docker.Mounts}}
      - {{.Host}}:{{.Container}}
{{- end}}
{{if .LLM}}
{{- if .LLM.HomeMounts}}
{{- range .LLM.HomeMounts}}
      - ./{{.HostDir}}:{{.ContainerDir}}
{{- end}}
{{- end}}
      - ./.renkin/conf:/renkin-conf:ro
{{- end}}
{{- end}}

{{- range .ToolList.Tools}}{{if eq .Type "mcp"}}
  {{.Name}}:
{{- if .BuildContext}}
    build:
      context: {{.BuildContext}}
{{- end}}
{{- if .Image}}
    image: {{.Image}}
{{- end}}
    ports:
      - "{{.Port}}:{{.Port}}"
{{- if .Environment}}
    environment:
{{- range .Environment}}
      - {{.}}
{{- end}}
{{- end}}
{{- if .Mounts}}
    volumes:
{{- range .Mounts}}
      - {{.Host}}:{{.Container}}
{{- end}}
{{- end}}
{{- if .HealthPath}}
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:{{.Port}}{{.HealthPath}}"]
      interval: 10s
      timeout: 5s
      retries: 5
{{- end}}
{{- end}}{{end}}
`

const envTemplate = `{{range .Entries}}{{.Key}}={{.Value}}
{{end}}`

type GeneratorData struct {
	config.Config
	EnvKeys              []string
	DefaultEnv           []string
	ProxyKeys            []string
	HealthTools          []config.Tool
	RuntimeConfigInstall string
}

type EnvEntry struct {
	Key   string
	Value string
}

func GenerateGeminiSettings(cfg config.Config) string {
	var servers []string
	for _, t := range cfg.ToolList.Tools {
		if t.MCPConfigGemini != "" {
			servers = append(servers, t.MCPConfigGemini)
		}
	}

	if len(servers) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("{\n  \"mcpServers\": {\n")
	for i, s := range servers {
		buf.WriteString(s)
		if i < len(servers)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  }\n}\n")
	return buf.String()
}

func GenerateAntigravityMCPConfig(cfg config.Config) string {
	return GenerateGeminiSettings(cfg)
}

func GenerateCodexConfig(cfg config.Config) string {
	var configs []string
	for _, t := range cfg.ToolList.Tools {
		if t.MCPConfigCodex != "" {
			configs = append(configs, t.MCPConfigCodex)
		}
	}

	if len(configs) == 0 {
		return ""
	}

	var buf bytes.Buffer
	for _, c := range configs {
		buf.WriteString(c)
		buf.WriteString("\n")
	}
	return buf.String()
}

func GenerateDockerfile(cfg config.Config) (string, error) {
	tmpl, err := template.New("Dockerfile").Parse(dockerfileTemplate)
	if err != nil {
		return "", err
	}
	data := GeneratorData{
		Config:               cfg,
		RuntimeConfigInstall: GenerateRuntimeConfigInstall(cfg),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func GenerateRuntimeConfigInstall(cfg config.Config) string {
	var startup []string
	if cfg.LLM != nil && cfg.LLM.Startup != "" {
		startup = append(startup, cfg.LLM.Startup)
	}
	for _, tool := range cfg.ToolList.Tools {
		if tool.Startup != "" {
			startup = append(startup, tool.Startup)
		}
	}
	if cfg.LLM == nil && len(startup) == 0 {
		return ""
	}

	var script bytes.Buffer
	script.WriteString("# Runtime LLM config generation\n")
	script.WriteString("RUN cat > /usr/local/bin/renkin-generate-llm-config <<'RENKIN_CONFIG_EOF'\n")
	script.WriteString("#!/usr/bin/env bash\n")
	script.WriteString("set -euo pipefail\n")
	script.WriteString("RENKIN_CODEX_CONFIG=\"${RENKIN_CODEX_CONFIG:-/tmp/renkin-codex-config.toml}\"\n")
	script.WriteString("RENKIN_GEMINI_MCP_SERVERS=\"${RENKIN_GEMINI_MCP_SERVERS:-/tmp/renkin-gemini-mcp-servers.jsonl}\"\n")
	script.WriteString("mkdir -p /root/.codex /root/.gemini\n")
	script.WriteString(": > \"$RENKIN_CODEX_CONFIG\"\n")
	script.WriteString(": > \"$RENKIN_GEMINI_MCP_SERVERS\"\n\n")
	script.WriteString("renkin_add_codex_config() {\n  cat >> \"$RENKIN_CODEX_CONFIG\"\n  printf '\\n' >> \"$RENKIN_CODEX_CONFIG\"\n}\n\n")
	script.WriteString("renkin_add_gemini_mcp_server() {\n  tmp=\"$(mktemp)\"\n  cat > \"$tmp\"\n  tr -d '\\n' < \"$tmp\" >> \"$RENKIN_GEMINI_MCP_SERVERS\"\n  printf '\\n' >> \"$RENKIN_GEMINI_MCP_SERVERS\"\n  rm -f \"$tmp\"\n}\n\n")

	// 1. Resolve and place workspace configs if they exist
	script.WriteString("# Resolve and place configs from /renkin-conf if they exist\n")
	if cfg.LLM != nil {
		for _, rc := range cfg.LLM.RuntimeConfigs {
			script.WriteString(fmt.Sprintf("if [ -f /renkin-conf/%s ]; then\n", rc.Source))
			script.WriteString(fmt.Sprintf("  mkdir -p %s\n", filepath.Dir(rc.Target)))
			script.WriteString(fmt.Sprintf("  python3 -c 'import os, sys; print(os.path.expandvars(sys.stdin.read()))' < /renkin-conf/%s > %s\n", rc.Source, rc.Target))
			script.WriteString("fi\n")
		}
	}
	script.WriteString("\n")

	// 2. Add servers from tools (legacy/startup way)
	for _, item := range startup {
		script.WriteString(item)
		if len(item) == 0 || item[len(item)-1] != '\n' {
			script.WriteByte('\n')
		}
		script.WriteByte('\n')
	}

	script.WriteString("if [ -s \"$RENKIN_CODEX_CONFIG\" ] && [ ! -f /root/.codex/config.toml ]; then\n")
	script.WriteString("  mkdir -p /root/.codex\n")
	script.WriteString("  cp \"$RENKIN_CODEX_CONFIG\" /root/.codex/config.toml\n")
	script.WriteString("fi\n")
	script.WriteString("if [ -s \"$RENKIN_GEMINI_MCP_SERVERS\" ] && [ ! -f /root/.gemini/settings.json ]; then\n")
	script.WriteString("  mkdir -p /root/.gemini\n")
	script.WriteString("  {\n")
	script.WriteString("    printf '{\\n  \"mcpServers\": {\\n'\n")
	script.WriteString("    first=1\n")
	script.WriteString("    while IFS= read -r server; do\n")
	script.WriteString("      [ -n \"$server\" ] || continue\n")
	script.WriteString("      if [ \"$first\" -eq 1 ]; then\n")
	script.WriteString("        printf '%s' \"$server\"\n")
	script.WriteString("        first=0\n")
	script.WriteString("      else\n")
	script.WriteString("        printf ',\\n%s' \"$server\"\n")
	script.WriteString("      fi\n")
	script.WriteString("    done < \"$RENKIN_GEMINI_MCP_SERVERS\"\n")
	script.WriteString("    if [ \"$first\" -eq 0 ]; then printf '\\n'; fi\n")
	script.WriteString("    printf '  }\\n}\\n'\n")
	script.WriteString("  } > /root/.gemini/settings.json\n")
	script.WriteString("fi\n")
	script.WriteString("RENKIN_CONFIG_EOF\n")
	script.WriteString("RUN chmod +x /usr/local/bin/renkin-generate-llm-config\n")

	return script.String()
}

func GenerateDockerCompose(cfg config.Config) (string, error) {
	tmpl, err := template.New("docker-compose.yml").Parse(dockerComposeTemplate)
	if err != nil {
		return "", err
	}

	data := GeneratorData{
		Config:      cfg,
		EnvKeys:     composeEnvKeys(collectAgentEnvKeys(cfg)),
		DefaultEnv:  []string{},
		ProxyKeys:   config.GetActiveProxyKeys(),
		HealthTools: mcpHealthTools(cfg),
	}

	if cfg.LLM != nil {
		data.DefaultEnv = append(data.DefaultEnv, cfg.LLM.DefaultEnv...)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func composeEnvKeys(keys []string) []string {
	filtered := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "AGENT_ID" || containsEnvAssignment(key) {
			continue
		}
		filtered = append(filtered, key)
	}
	return filtered
}

func collectAgentEnvKeys(cfg config.Config) []string {
	var envKeys []string
	if cfg.LLM != nil {
		envKeys = append(envKeys, cfg.LLM.GetEnvKeys()...)
		envKeys = append(envKeys, "AGENT_ID")
	}
	envKeys = append(envKeys, config.GetActiveProxyKeys()...)
	for _, t := range cfg.ToolList.Tools {
		if t.Type == "shell" {
			envKeys = append(envKeys, t.Environment...)
		}
	}

	seen := make(map[string]bool)
	var uniqueKeys []string
	for _, key := range envKeys {
		if containsEnvAssignment(key) || seen[key] {
			continue
		}
		seen[key] = true
		uniqueKeys = append(uniqueKeys, key)
	}
	return uniqueKeys
}

func mcpHealthTools(cfg config.Config) []config.Tool {
	var tools []config.Tool
	for _, tool := range cfg.ToolList.Tools {
		if tool.Type == "mcp" && tool.HealthPath != "" {
			tools = append(tools, tool)
		}
	}
	return tools
}

func containsEnvAssignment(key string) bool {
	return strings.Contains(key, "=")
}

func GenerateEnv(cfg config.Config) (string, error) {
	return GenerateEnvWithAgentID(cfg, "")
}

func GenerateEnvWithAgentID(cfg config.Config, agentID string) (string, error) {
	tmpl, err := template.New(".env").Parse(envTemplate)
	if err != nil {
		return "", err
	}

	envKeys := cfg.CollectEnvKeys()
	if len(envKeys) == 0 {
		return "", nil
	}

	entries := make([]EnvEntry, 0, len(envKeys))
	for _, key := range envKeys {
		value := ""
		if key == "AGENT_ID" {
			value = agentID
		}
		entries = append(entries, EnvEntry{Key: key, Value: value})
	}

	data := struct {
		Entries []EnvEntry
	}{
		Entries: entries,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
