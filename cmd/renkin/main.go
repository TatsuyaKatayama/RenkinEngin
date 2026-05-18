package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/docker"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/utils"
	"github.com/spf13/cobra"
)

var (
	dockerPath string
	llmPath    string
	toolsPath  []string
	skillsPath string
)

func main() {
	var rootCmd = &cobra.Command{Use: "renkin"}

	var assignCmd = &cobra.Command{
		Use:   "assign <target_dir>",
		Short: "Generate Dockerfile, docker-compose.yml, and other artifacts",
		Args:  cobra.ExactArgs(1),
		RunE:  runAssign,
	}

	assignCmd.Flags().StringVar(&dockerPath, "docker", "", "Docker infra config file")
	assignCmd.Flags().StringVar(&llmPath, "llm", "", "LLM config file or preset name")
	assignCmd.Flags().StringSliceVar(&toolsPath, "tools", []string{}, "Comma-separated list of tool presets or file paths")
	assignCmd.Flags().StringVar(&skillsPath, "skills", "", "LLM instructions file")

	var startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the docker-compose environment and attach to LLM agent",
		RunE:  runStart,
	}

	var endCmd = &cobra.Command{
		Use:   "end",
		Short: "Stop and remove the docker-compose environment",
		RunE:  runEnd,
	}

	rootCmd.AddCommand(assignCmd, startCmd, endCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runAssign(cmd *cobra.Command, args []string) error {
	targetDir := args[0]
	if err := utils.EnsureDir(targetDir); err != nil {
		return err
	}

	// Auto-discovery of config files if not specified
	if dockerPath == "" {
		p := filepath.Join(targetDir, "docker.conf")
		if _, err := os.Stat(p); err == nil {
			dockerPath = p
		} else {
			// Fallback to default preset
			presetsDockerDir := "presets/docker"
			if _, err := os.Stat(presetsDockerDir); os.IsNotExist(err) {
				if exePath, err := os.Executable(); err == nil {
					presetsDockerDir = filepath.Join(filepath.Dir(exePath), "presets/docker")
				}
			}
			presetPath := filepath.Join(presetsDockerDir, "default.toml")
			if _, err := os.Stat(presetPath); err == nil {
				dockerPath = presetPath
				fmt.Println("Using default docker preset")
			} else {
				return fmt.Errorf("docker.conf not found in %s and default preset not found", targetDir)
			}
		}
	}

	if llmPath == "" {
		p := filepath.Join(targetDir, "llm.conf")
		if _, err := os.Stat(p); err == nil {
			llmPath = p
		}
	}

	if len(toolsPath) == 0 {
		p := filepath.Join(targetDir, "tool_list.toml")
		if _, err := os.Stat(p); err == nil {
			toolsPath = []string{p}
		}
	}

	if skillsPath == "" {
		p := filepath.Join(targetDir, "skills.md")
		if _, err := os.Stat(p); err == nil {
			skillsPath = p
		}
	}

	dConf, err := config.LoadDockerConf(dockerPath)
	if err != nil {
		return err
	}

	var lConf *config.LLMConf
	if llmPath != "" {
		// Resolve LLM preset
		// 1. Check if the path exists directly
		if _, err := os.Stat(llmPath); err == nil {
			lConf, err = config.LoadLLMConf(llmPath)
			if err != nil {
				return err
			}
		} else {
			// 2. Try to resolve as a preset name
			presetsLLMDir := "presets/llms"
			if _, err := os.Stat(presetsLLMDir); os.IsNotExist(err) {
				if exePath, err := os.Executable(); err == nil {
					presetsLLMDir = filepath.Join(filepath.Dir(exePath), "presets/llms")
				}
			}
			presetPath := filepath.Join(presetsLLMDir, llmPath+".toml")
			if _, err := os.Stat(presetPath); err == nil {
				lConf, err = config.LoadLLMConf(presetPath)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("llm config or preset not found: %s", llmPath)
			}
		}
	}

	var tList config.ToolList
	// 3. Resolve tool inputs
	presetsDir := "presets/tools"
	if _, err := os.Stat(presetsDir); os.IsNotExist(err) {
		if exePath, err := os.Executable(); err == nil {
			presetsDir = filepath.Join(filepath.Dir(exePath), "presets/tools")
		}
	}

	for _, input := range toolsPath {
		var list config.ToolList
		if _, err := os.Stat(input); err == nil {
			// It's a file path
			list, err = config.LoadToolList(input)
			if err != nil {
				return err
			}
		} else {
			// Treat as preset name
			list = config.ToolList{Tools: []config.Tool{{Preset: input}}}
		}
		tList.Tools = append(tList.Tools, list.Tools...)
	}

	if err := tList.ResolvePresets(presetsDir); err != nil {
		return fmt.Errorf("failed to resolve presets: %v", err)
	}

	cfg := config.Config{
		Docker:   dConf,
		LLM:      lConf,
		ToolList: tList,
	}

	// Generate Dockerfile
	df, err := generator.GenerateDockerfile(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "Dockerfile"), []byte(df), 0644); err != nil {
		return err
	}

	// Generate docker-compose.yml
	dc, err := generator.GenerateDockerCompose(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "docker-compose.yml"), []byte(dc), 0644); err != nil {
		return err
	}

	// Generate .env
	env, err := generator.GenerateEnv(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, ".env"), []byte(env), 0644); err != nil {
		return err
	}

	// Create workspace and synthesize skills
	workspaceDir := filepath.Join(targetDir, "workspace")
	if err := utils.EnsureDir(workspaceDir); err != nil {
		return err
	}

	var llmCmd string
	if lConf != nil {
		llmCmd = lConf.Cmd
		skillName, err := lConf.GetSkillFileName()
		if err != nil {
			return err
		}

		var aggregatedSkills strings.Builder

		// 1. Add tool instructions
		for _, t := range cfg.ToolList.Tools {
			if t.Instructions != "" {
				aggregatedSkills.WriteString(fmt.Sprintf("## %s Instructions\n", t.Name))
				aggregatedSkills.WriteString(t.Instructions)
				aggregatedSkills.WriteString("\n\n")
			}
		}

		// 2. Add base skills from file if provided
		if skillsPath != "" {
			content, err := os.ReadFile(skillsPath)
			if err != nil {
				return err
			}
			aggregatedSkills.WriteString("## Base Skills\n")
			aggregatedSkills.Write(content)
			aggregatedSkills.WriteString("\n")
		}

		if aggregatedSkills.Len() > 0 {
			if err := os.WriteFile(filepath.Join(workspaceDir, skillName), []byte(aggregatedSkills.String()), 0644); err != nil {
				return err
			}
		}
	}

	// Save metadata
	meta := struct {
		LLMCmd string `toml:"llm_cmd"`
	}{LLMCmd: llmCmd}
	if err := config.SaveMetadata(filepath.Join(targetDir, ".renkin_metadata.toml"), meta); err != nil {
		return err
	}

	fmt.Printf("Successfully generated artifacts in %s\n", targetDir)
	return nil
}

func runStart(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat("docker-compose.yml"); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found. Please run 'renkin assign' first")
	}

	fmt.Println("Starting containers...")
	if err := docker.ComposeUp(); err != nil {
		return err
	}

	metadataPath := ".renkin_metadata.toml"
	if _, err := os.Stat(metadataPath); err == nil {
		var meta struct {
			LLMCmd string `toml:"llm_cmd"`
		}
		if err := config.LoadMetadata(metadataPath, &meta); err == nil && meta.LLMCmd != "" {
			fmt.Printf("Attaching to LLM agent with command: %s\n", meta.LLMCmd)
			return docker.ExecAttach("llm-agent", meta.LLMCmd)
		}
	}

	fmt.Println("Containers started. No LLM agent to attach.")
	return nil
}

func runEnd(cmd *cobra.Command, args []string) error {
	fmt.Println("Stopping containers...")
	return docker.ComposeDown()
}
