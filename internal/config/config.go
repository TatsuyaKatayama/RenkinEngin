package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type DockerConf struct {
	BaseImage    string  `toml:"base_image"`
	Mounts       []Mount `toml:"mount"`
	Instructions string  `toml:"instructions"`
}

type Mount struct {
	Host      string `toml:"host"`
	Container string `toml:"container"`
}

type AuthMountConf struct {
	HostPath      string `toml:"host_path"`
	ContainerPath string `toml:"container_path"`
}

type RuntimeConfig struct {
	Source string `toml:"source"`
	Target string `toml:"target"`
}

type LLMConf struct {
	Cmd               string          `toml:"cmd"`
	Install           string          `toml:"install"`
	Startup           string          `toml:"startup"`
	Ports             []string        `toml:"ports"`
	SkillFile         string          `toml:"skill_file"`
	EnvKeys           []string        `toml:"env_keys"`
	DefaultEnv        []string        `toml:"default_env"`
	RuntimeConfigs    []RuntimeConfig `toml:"runtime_configs"`
	AuthMount         *AuthMountConf  `toml:"auth_mount"`
	MCPConfigTarget   string          `toml:"mcp_config_target"`
	AgentConfigTarget string          `toml:"agent_config_target"`
}

type Tool struct {
	Name             string   `toml:"name"`
	Type             string   `toml:"type"`
	Preset           string   `toml:"preset"`
	Install          string   `toml:"install"`
	Startup          string   `toml:"startup"`
	MCPConfigGemini  string   `toml:"mcp_config_gemini"`
	MCPConfigCodex   string   `toml:"mcp_config_codex"`
	Instructions     string   `toml:"instructions"`
	Image            string   `toml:"image"`
	Port             int      `toml:"port"`
	Environment      []string `toml:"environment"`
}

type ToolList struct {
	Tools []Tool `toml:"tool"`
}

type Config struct {
	Docker   DockerConf
	LLM      *LLMConf
	ToolList ToolList
}

func LoadDockerConf(path string) (DockerConf, error) {
	var conf DockerConf
	conf.BaseImage = "ubuntu:24.04" // Default
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		return conf, err
	}
	return conf, nil
}

func LoadLLMConf(path string) (*LLMConf, error) {
	var conf LLMConf
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		return nil, err
	}
	if conf.Cmd == "" {
		return nil, fmt.Errorf("llm.conf: cmd is required")
	}
	return &conf, nil
}

func LoadToolList(path string) (ToolList, error) {
	var list ToolList
	if _, err := toml.DecodeFile(path, &list); err != nil {
		return list, err
	}
	return list, nil
}

func (tl *ToolList) ResolvePresets(presetsDir string) error {
	resolvedTools, err := resolveTools(tl.Tools, presetsDir, map[string]bool{})
	if err != nil {
		return err
	}

	var uniqueTools []Tool
	seenNames := make(map[string]bool)
	for _, t := range resolvedTools {
		if !seenNames[t.Name] {
			uniqueTools = append(uniqueTools, t)
			seenNames[t.Name] = true
		}
	}

	tl.Tools = uniqueTools
	return tl.validate()
}

func resolveTools(tools []Tool, presetsDir string, resolving map[string]bool) ([]Tool, error) {
	var resolvedTools []Tool
	for _, t := range tools {
		var toolsToResolve []Tool
		if t.Preset != "" {
			if resolving[t.Preset] {
				return nil, fmt.Errorf("cyclic preset reference: %s", t.Preset)
			}
			resolving[t.Preset] = true

			presetTools, err := loadPresetTools(presetsDir, t.Preset)
			if err != nil {
				delete(resolving, t.Preset)
				return nil, err
			}

			nestedTools, err := resolveTools(presetTools.Tools, presetsDir, resolving)
			delete(resolving, t.Preset)
			if err != nil {
				return nil, err
			}

			if len(nestedTools) == 1 {
				applyToolOverrides(&nestedTools[0], t)
			} else if hasToolOverrides(t) {
				return nil, fmt.Errorf("preset %s resolves to multiple tools and cannot be overridden", t.Preset)
			}
			toolsToResolve = append(toolsToResolve, nestedTools...)
		} else {
			toolsToResolve = append(toolsToResolve, t)
		}

		resolvedTools = append(resolvedTools, toolsToResolve...)
	}
	return resolvedTools, nil
}

func hasToolOverrides(t Tool) bool {
	return t.Name != "" ||
		t.Type != "" ||
		t.Install != "" ||
		t.Startup != "" ||
		t.Instructions != "" ||
		t.Image != "" ||
		t.Port != 0 ||
		len(t.Environment) > 0
}

func loadPresetTools(presetsDir string, preset string) (ToolList, error) {
	presetPath := filepath.Join(presetsDir, preset+".toml")
	if _, err := os.Stat(presetPath); os.IsNotExist(err) {
		return ToolList{}, fmt.Errorf("preset %s not found in %s", preset, presetsDir)
	}

	var presetTools ToolList
	if _, err := toml.DecodeFile(presetPath, &presetTools); err != nil {
		return ToolList{}, fmt.Errorf("failed to parse preset %s: %v", preset, err)
	}

	if len(presetTools.Tools) == 0 {
		return ToolList{}, fmt.Errorf("preset %s contains no tools", preset)
	}
	return presetTools, nil
}

func applyToolOverrides(pt *Tool, t Tool) {
	if t.Name != "" {
		pt.Name = t.Name
	}
	if t.Type != "" {
		pt.Type = t.Type
	}
	if t.Install != "" {
		pt.Install = t.Install
	}
	if t.Startup != "" {
		pt.Startup = t.Startup
	}
	if t.MCPConfigGemini != "" {
		pt.MCPConfigGemini = t.MCPConfigGemini
	}
	if t.MCPConfigCodex != "" {
		pt.MCPConfigCodex = t.MCPConfigCodex
	}
	if t.Instructions != "" {
		pt.Instructions = t.Instructions
	}
	if t.Image != "" {
		pt.Image = t.Image
	}
	if t.Port != 0 {
		pt.Port = t.Port
	}
	pt.Environment = append(pt.Environment, t.Environment...)
}

func (tl ToolList) validate() error {
	for _, t := range tl.Tools {
		if t.Type == "shell" && t.Install == "" {
			return fmt.Errorf("tool %s: install is required for shell type", t.Name)
		}
		if t.Type == "mcp" && (t.Image == "" || t.Port == 0) {
			return fmt.Errorf("tool %s: image and port are required for mcp type", t.Name)
		}
	}
	return nil
}

func (l *LLMConf) GetType() (string, error) {
	parts := strings.Fields(l.Cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid llm cmd")
	}
	return parts[0], nil
}

func (l *LLMConf) GetSkillFileName() (string, error) {
	if l.SkillFile != "" {
		return l.SkillFile, nil
	}
	llmType, err := l.GetType()
	if err != nil {
		return "", err
	}
	switch llmType {
	case "gemini", "agy":
		return "GEMINI.md", nil
	case "codex":
		return "AGENTS.md", nil
	default:
		return "AGENTS.md", nil
	}
}

func (l *LLMConf) GetEnvKeys() []string {
	if len(l.EnvKeys) > 0 {
		return l.EnvKeys
	}
	llmType, _ := l.GetType()
	switch llmType {
	case "gemini", "agy":
		return []string{"GEMINI_API_KEY"}
	case "codex":
		return []string{"OPENAI_API_KEY"}
	default:
		return []string{}
	}
}

func GetHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (c *Config) CollectEnvKeys() []string {
	var envKeys []string
	if c.LLM != nil {
		envKeys = append(envKeys, c.LLM.GetEnvKeys()...)
	}
	envKeys = append(envKeys, GetActiveProxyKeys()...)
	for _, t := range c.ToolList.Tools {
		envKeys = append(envKeys, t.Environment...)
	}

	seen := make(map[string]bool)
	var uniqueKeys []string
	for _, key := range envKeys {
		if seen[key] {
			continue
		}
		seen[key] = true
		uniqueKeys = append(uniqueKeys, key)
	}
	return uniqueKeys
}

func GetActiveProxyKeys() []string {
	proxyEnvNames := []string{
		"HTTP_PROXY", "http_proxy",
		"HTTPS_PROXY", "https_proxy",
		"NO_PROXY", "no_proxy",
	}
	var keys []string
	for _, name := range proxyEnvNames {
		if os.Getenv(name) != "" {
			keys = append(keys, name)
		}
	}
	return keys
}

type Metadata struct {
	LLMCmd  string   `toml:"llm_cmd"`
	EnvKeys []string `toml:"env_keys"`
}

func SaveMetadata(path string, meta Metadata) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(meta)
}

func LoadMetadata(path string, meta *Metadata) error {
	if _, err := toml.DecodeFile(path, meta); err != nil {
		return err
	}
	return nil
}
