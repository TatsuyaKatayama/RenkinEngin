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

type RuntimeConfig struct {
	Source string `toml:"source"`
	Target string `toml:"target"`
}

type HomeMount struct {
	HostDir      string `toml:"host_dir"`
	ContainerDir string `toml:"container_dir"`
}

type LLMConf struct {
	Cmd            string          `toml:"cmd"`
	Install        string          `toml:"install"`
	Startup        string          `toml:"startup"`
	Ports          []string        `toml:"ports"`
	SkillFile      string          `toml:"skill_file"`
	EnvKeys        []string        `toml:"env_keys"`
	DefaultEnv     []string        `toml:"default_env"`
	RuntimeConfigs []RuntimeConfig `toml:"runtime_configs"`
	HomeMounts     []HomeMount     `toml:"home_mounts"`
	LoopCmd        string          `toml:"loop_cmd"`
	RestartPolicy  string          `toml:"restart_policy"`
	RestartDelay   int             `toml:"restart_delay"`
	LogDir         string          `toml:"log_dir"`
	StdoutLog      string          `toml:"stdout_log"`
	StderrLog      string          `toml:"stderr_log"`
}

type Tool struct {
	Name            string   `toml:"name"`
	Type            string   `toml:"type"`
	Preset          string   `toml:"preset"`
	Install         string   `toml:"install"`
	Startup         string   `toml:"startup"`
	MCPConfigGemini string   `toml:"mcp_config_gemini"`
	MCPConfigCodex  string   `toml:"mcp_config_codex"`
	Instructions    string   `toml:"instructions"`
	Image           string   `toml:"image"`
	BuildContext    string   `toml:"build_context"`
	Port            int      `toml:"port"`
	HealthPath      string   `toml:"health_path"`
	Environment     []string `toml:"environment"`
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
	seenKeys := make(map[string]bool)
	for _, t := range resolvedTools {
		key := toolIdentityKey(t)
		if !seenKeys[key] {
			uniqueTools = append(uniqueTools, t)
			seenKeys[key] = true
		}
	}

	tl.Tools = uniqueTools
	return tl.validate()
}

func toolIdentityKey(t Tool) string {
	switch {
	case t.Name != "":
		return "name:" + t.Name
	case t.Preset != "":
		return "preset:" + t.Preset
	case t.MCPConfigCodex != "":
		return "mcp-codex:" + t.MCPConfigCodex
	case t.MCPConfigGemini != "":
		return "mcp-gemini:" + t.MCPConfigGemini
	case t.Image != "":
		return "image:" + t.Image
	case t.BuildContext != "":
		return "build-context:" + t.BuildContext
	case t.Install != "":
		return "install:" + t.Install
	default:
		return fmt.Sprintf("tool:%s:%s:%s:%d", t.Type, t.Image, t.BuildContext, t.Port)
	}
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
		t.BuildContext != "" ||
		t.Port != 0 ||
		t.HealthPath != "" ||
		len(t.Environment) > 0
}

type ToolPresetData struct {
	ToolList        ToolList
	BotPrompt       string
	BotLoop         string
	Instructions    string
	MCPConfigGemini string
	MCPConfigCodex  string
	RestartPolicy   string
	RestartDelay    int
}

const defaultAgentID = "default-agent"

func RenderLoopTemplate(loopTemplate string, llmConf *LLMConf, targetDir string) string {
	loopTemplate = strings.ReplaceAll(loopTemplate, defaultAgentID, ResolveDefaultAgentID(targetDir))
	return strings.ReplaceAll(loopTemplate, "{llm_cmd}", renderLoopCommand(llmConf))
}

func ResolveDefaultAgentID(targetDir string) string {
	targetBase := filepath.Clean(targetDir)
	if targetBase == "." || targetBase == "/" {
		wd, _ := os.Getwd()
		return filepath.Base(wd)
	}
	return filepath.Base(targetBase)
}

func renderLoopCommand(llmConf *LLMConf) string {
	if llmConf == nil {
		return ""
	}
	return strings.ReplaceAll(llmConf.LoopCmd, "{session_id}", `"${AGENT_ID}-session"`)
}

func LoadLLMPreset(presetsDir string, presetName string) (*LLMConf, string, string, string, error) {
	dirPath := filepath.Join(presetsDir, presetName)
	fi, err := os.Stat(dirPath)
	if err == nil && fi.IsDir() {
		// Loaded as directory preset
		tomlPath := filepath.Join(dirPath, "llm.toml")
		conf, err := LoadLLMConf(tomlPath)
		if err != nil {
			return nil, "", "", "", err
		}

		var botPrompt, botLoop, skills string
		if b, err := os.ReadFile(filepath.Join(dirPath, "bot_prompt.md")); err == nil {
			botPrompt = string(b)
		}
		if b, err := os.ReadFile(filepath.Join(dirPath, "bot-loop.sh")); err == nil {
			botLoop = string(b)
		}
		if b, err := os.ReadFile(filepath.Join(dirPath, "skills.md")); err == nil {
			skills = string(b)
		}

		return conf, botPrompt, botLoop, skills, nil
	}

	// Fallback to single TOML file
	tomlPath := filepath.Join(presetsDir, presetName+".toml")
	conf, err := LoadLLMConf(tomlPath)
	if err != nil {
		return nil, "", "", "", err
	}
	return conf, "", "", "", nil
}

func LoadToolPreset(presetsDir string, presetName string) (ToolPresetData, error) {
	dirPath := filepath.Join(presetsDir, presetName)
	fi, err := os.Stat(dirPath)
	if err == nil && fi.IsDir() {
		// Loaded as directory preset
		tomlPath := filepath.Join(dirPath, "tool.toml")

		var presetTools ToolList
		if _, err := toml.DecodeFile(tomlPath, &presetTools); err != nil {
			return ToolPresetData{}, fmt.Errorf("failed to parse preset tool.toml %s: %v", presetName, err)
		}

		var botPrompt, botLoop, instructions, mcpGemini, mcpCodex string
		var restartPolicy string
		var restartDelay int
		if b, err := os.ReadFile(filepath.Join(dirPath, "bot_prompt.md")); err == nil {
			botPrompt = string(b)
		}
		if b, err := os.ReadFile(filepath.Join(dirPath, "bot-loop.sh")); err == nil {
			botLoop = string(b)
		}
		if b, err := os.ReadFile(filepath.Join(dirPath, "instructions.md")); err == nil {
			instructions = string(b)
		}
		if b, err := os.ReadFile(filepath.Join(dirPath, "mcp_config_gemini.json")); err == nil {
			mcpGemini = string(b)
		}
		if b, err := os.ReadFile(filepath.Join(dirPath, "mcp_config_codex.toml")); err == nil {
			mcpCodex = string(b)
		}
		var presetMeta struct {
			RestartPolicy string `toml:"restart_policy"`
			RestartDelay  int    `toml:"restart_delay"`
		}
		if _, err := toml.DecodeFile(tomlPath, &presetMeta); err == nil {
			restartPolicy = presetMeta.RestartPolicy
			restartDelay = presetMeta.RestartDelay
		}

		// Inject files into the parsed tool list if they exist
		for i := range presetTools.Tools {
			if instructions != "" {
				presetTools.Tools[i].Instructions = instructions
			}
			if presetTools.Tools[i].Preset != "" {
				continue
			}
			if mcpGemini != "" {
				presetTools.Tools[i].MCPConfigGemini = mcpGemini
			}
			if mcpCodex != "" {
				presetTools.Tools[i].MCPConfigCodex = mcpCodex
			}
		}

		return ToolPresetData{
			ToolList:        presetTools,
			BotPrompt:       botPrompt,
			BotLoop:         botLoop,
			Instructions:    instructions,
			MCPConfigGemini: mcpGemini,
			MCPConfigCodex:  mcpCodex,
			RestartPolicy:   restartPolicy,
			RestartDelay:    restartDelay,
		}, nil
	}

	// Fallback to single TOML file
	tomlPath := filepath.Join(presetsDir, presetName+".toml")
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		return ToolPresetData{}, fmt.Errorf("preset %s not found in %s", presetName, presetsDir)
	}

	var presetTools ToolList
	if _, err := toml.DecodeFile(tomlPath, &presetTools); err != nil {
		return ToolPresetData{}, fmt.Errorf("failed to parse preset %s: %v", presetName, err)
	}

	return ToolPresetData{
		ToolList: presetTools,
	}, nil
}

func loadPresetTools(presetsDir string, preset string) (ToolList, error) {
	tpData, err := LoadToolPreset(presetsDir, preset)
	if err != nil {
		return ToolList{}, err
	}
	if len(tpData.ToolList.Tools) == 0 {
		return ToolList{}, fmt.Errorf("preset %s contains no tools", preset)
	}
	return tpData.ToolList, nil
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
	if t.BuildContext != "" {
		pt.BuildContext = t.BuildContext
	}
	if t.Port != 0 {
		pt.Port = t.Port
	}
	if t.HealthPath != "" {
		pt.HealthPath = t.HealthPath
	}
	pt.Environment = append(pt.Environment, t.Environment...)
}

func (tl ToolList) validate() error {
	for _, t := range tl.Tools {
		if t.Type == "shell" && t.Install == "" {
			return fmt.Errorf("tool %s: install is required for shell type", t.Name)
		}
		if t.Type == "mcp" && ((t.Image == "" && t.BuildContext == "") || t.Port == 0) {
			return fmt.Errorf("tool %s: image or build_context and port are required for mcp type", t.Name)
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
		envKeys = append(envKeys, "AGENT_ID")
	}
	envKeys = append(envKeys, GetActiveProxyKeys()...)
	for _, t := range c.ToolList.Tools {
		envKeys = append(envKeys, t.Environment...)
	}

	seen := make(map[string]bool)
	var uniqueKeys []string
	for _, key := range envKeys {
		if strings.Contains(key, "=") {
			continue
		}
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
	LLMCmd        string   `toml:"llm_cmd"`
	EnvKeys       []string `toml:"env_keys"`
	LoopCmd       string   `toml:"loop_cmd"`
	RestartPolicy string   `toml:"restart_policy"`
	RestartDelay  int      `toml:"restart_delay"`
	LogDir        string   `toml:"log_dir"`
	StdoutLog     string   `toml:"stdout_log"`
	StderrLog     string   `toml:"stderr_log"`
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
