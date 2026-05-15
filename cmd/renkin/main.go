package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/docker"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/utils"
	"github.com/spf13/cobra"
)

var (
	dockerPath string
	llmPath    string
	toolsPath  string
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

	assignCmd.Flags().StringVar(&dockerPath, "docker", "", "Docker infra config file (required)")
	assignCmd.Flags().StringVar(&llmPath, "llm", "", "LLM config file")
	assignCmd.Flags().StringVar(&toolsPath, "tools", "", "Tool list file (required)")
	assignCmd.Flags().StringVar(&skillsPath, "skills", "", "LLM instructions file")

	assignCmd.MarkFlagRequired("docker")
	assignCmd.MarkFlagRequired("tools")

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

	dConf, err := config.LoadDockerConf(dockerPath)
	if err != nil {
		return err
	}

	var lConf *config.LLMConf
	if llmPath != "" {
		lConf, err = config.LoadLLMConf(llmPath)
		if err != nil {
			return err
		}
	}

	tList, err := config.LoadToolList(toolsPath)
	if err != nil {
		return err
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
	if env != "" {
		if err := os.WriteFile(filepath.Join(targetDir, ".env"), []byte(env), 0644); err != nil {
			return err
		}
	}

	// Create workspace and copy skills
	workspaceDir := filepath.Join(targetDir, "workspace")
	if err := utils.EnsureDir(workspaceDir); err != nil {
		return err
	}

	var llmCmd string
	if lConf != nil {
		llmCmd = lConf.Cmd
		if skillsPath != "" {
			skillName, err := lConf.GetSkillFileName()
			if err != nil {
				return err
			}
			if err := utils.CopyFile(skillsPath, filepath.Join(workspaceDir, skillName)); err != nil {
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
