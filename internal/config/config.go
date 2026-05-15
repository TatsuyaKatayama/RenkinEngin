package config

import (
	"fmt"
	"os"
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
	Cmd      string `toml:"cmd"`
	AuthMode string `toml:"auth_mode"`
	Install  string `toml:"install"`
}

type Tool struct {
	Name    string `toml:"name"`
	Type    string `toml:"type"`
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
	for _, t := range list.Tools {
		if t.Type == "shell" && t.Install == "" {
			return list, fmt.Errorf("tool %s: install is required for shell type", t.Name)
		}
		if t.Type == "mcp" && (t.Image == "" || t.Port == 0) {
			return list, fmt.Errorf("tool %s: image and port are required for mcp type", t.Name)
		}
	}
	return list, nil
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
