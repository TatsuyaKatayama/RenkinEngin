package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type DockerConf struct {
	BaseImage string  `toml:"base_image"`
	Mounts    []Mount `toml:"mount"`
}

type Mount struct {
	Host      string `toml:"host"`
	Container string `toml:"container"`
}

type LLMConf struct {
	Cmd      string   `toml:"cmd"`
	AuthMode string   `toml:"auth_mode"`
	Install  string   `toml:"install"`
	Ports    []string `toml:"ports"`
}

type Tool struct {
	Name         string   `toml:"name"`
	Type         string   `toml:"type"`
	Preset       string   `toml:"preset"`
	Install      string   `toml:"install"`
	Instructions string   `toml:"instructions"`
	Image        string   `toml:"image"`
	Port         int      `toml:"port"`
	Environment  []string `toml:"environment"`
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
	conf.AuthMode = "api_key" // Default
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		return nil, err
	}
	if conf.Cmd == "" {
		return nil, fmt.Errorf("llm.conf: cmd is required")
	}
	if conf.AuthMode != "api_key" && conf.AuthMode != "browser" {
		return nil, fmt.Errorf("llm.conf: invalid auth_mode: %s", conf.AuthMode)
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

	seenNames := make(map[string]bool)
	for _, t := range resolvedTools {
		if seenNames[t.Name] {
			return fmt.Errorf("duplicate tool name: %s", t.Name)
		}
		seenNames[t.Name] = true
	}

	tl.Tools = resolvedTools
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
	llmType, err := l.GetType()
	if err != nil {
		return "", err
	}
	switch llmType {
	case "claude":
		return "CLAUDE.md", nil
	case "gemini":
		return "GEMINI.md", nil
	case "codex", "opencode":
		return "AGENTS.md", nil
	default:
		return "AGENTS.md", nil
	}
}

func (l *LLMConf) GetEnvKeys() []string {
	llmType, _ := l.GetType()
	switch llmType {
	case "claude":
		return []string{"ANTHROPIC_API_KEY"}
	case "gemini":
		return []string{"GEMINI_API_KEY"}
	case "codex":
		return []string{"OPENAI_API_KEY"}
	case "opencode":
		return []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY"}
	default:
		return []string{}
	}
}

func GetHomeDir() (string, error) {
	return os.UserHomeDir()
}

func SaveMetadata(path string, meta interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(meta)
}

func LoadMetadata(path string, meta interface{}) error {
	if _, err := toml.DecodeFile(path, meta); err != nil {
		return err
	}
	return nil
}
