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
	Name    string `toml:"name"`
	Type    string `toml:"type"`
	Preset  string `toml:"preset"`  // New field
	Install string `toml:"install"` // For shell type
	Image   string `toml:"image"`   // For mcp type
	Port    int    `toml:"port"`    // For mcp type
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
	for i, t := range tl.Tools {
		if t.Preset != "" {
			presetPath := filepath.Join(presetsDir, t.Preset+".toml")
			if _, err := os.Stat(presetPath); os.IsNotExist(err) {
				return fmt.Errorf("preset %s not found in %s", t.Preset, presetsDir)
			}

			var presetTools ToolList
			if _, err := toml.DecodeFile(presetPath, &presetTools); err != nil {
				return fmt.Errorf("failed to parse preset %s: %v", t.Preset, err)
			}

			if len(presetTools.Tools) == 0 {
				return fmt.Errorf("preset %s contains no tools", t.Preset)
			}

			// Merge preset content into the tool definition
			pTool := presetTools.Tools[0]
			if tl.Tools[i].Name == "" {
				tl.Tools[i].Name = pTool.Name
			}
			if tl.Tools[i].Type == "" {
				tl.Tools[i].Type = pTool.Type
			}
			if tl.Tools[i].Install == "" {
				tl.Tools[i].Install = pTool.Install
			}
			if tl.Tools[i].Image == "" {
				tl.Tools[i].Image = pTool.Image
			}
			if tl.Tools[i].Port == 0 {
				tl.Tools[i].Port = pTool.Port
			}
		}

		// Validation after resolution
		if tl.Tools[i].Type == "shell" && tl.Tools[i].Install == "" {
			return fmt.Errorf("tool %s: install is required for shell type", tl.Tools[i].Name)
		}
		if tl.Tools[i].Type == "mcp" && (tl.Tools[i].Image == "" || tl.Tools[i].Port == 0) {
			return fmt.Errorf("tool %s: image and port are required for mcp type", tl.Tools[i].Name)
		}
	}
	return nil
}

func (l *LLMConf) GetType() (string, error) {
	parts := strings.Fields(l.Cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid llm cmd")
	}
	switch parts[0] {
	case "claude", "gemini", "codex", "opencode":
		return parts[0], nil
	default:
		return "", fmt.Errorf("unknown llm type: %s", parts[0])
	}
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
