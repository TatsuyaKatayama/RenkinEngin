package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TatsuyaKatayama/RenkinEngin/internal/config"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/docker"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/generator"
	"github.com/TatsuyaKatayama/RenkinEngin/internal/utils"
	"github.com/spf13/cobra"
)

var (
	dockerPath  string
	llmPath     string
	toolsPath   []string
	skillsPath  string
	overrideCmd string
	noConfig    bool
	loopCmd     string
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
		Long: `Start the docker-compose environment and run the LLM agent.

If --loop is specified, it runs the LLM agent or custom script in an automated loop mode.
Any relative paths (e.g., ./work.sh) are resolved relative to the workspace root inside the container.
To stop/kill the loop, press Ctrl+C in your terminal or run 'renkin stop' from another terminal.

Note: If --cmd is provided, it takes absolute priority and overrides any default commands or loop script execution.`,
		RunE: runStart,
	}
	startCmd.Flags().StringVar(&overrideCmd, "cmd", "", "Override default LLM command (e.g., --cmd bash)")
	startCmd.Flags().BoolVar(&noConfig, "no-config", false, "Skip automatic configuration generation")
	startCmd.Flags().StringVar(&loopCmd, "loop", "", "Run in loop mode using default loop command or custom script")
	startCmd.Flags().Lookup("loop").NoOptDefVal = "default"

	var stopCmd = &cobra.Command{
		Use:   "stop",
		Short: "Stop and remove the docker-compose environment (including volumes)",
		RunE:  runStop,
	}

	var restartCmd = &cobra.Command{
		Use:   "restart",
		Short: "Restart the docker-compose environment",
		RunE:  runRestart,
	}
	restartCmd.Flags().StringVar(&overrideCmd, "cmd", "", "Override default LLM command (e.g., --cmd bash)")
	var rebuild bool
	restartCmd.Flags().BoolVar(&rebuild, "rebuild", false, "Rebuild images without cache before starting")

	var kaikoCmd = &cobra.Command{
		Use:   "kaiko",
		Short: "Completely remove the docker-compose environment, including images and volumes",
		RunE:  runKaiko,
	}
	var force bool
	kaikoCmd.Flags().BoolVarP(&force, "yes", "y", false, "Skip confirmation prompt")

	var toolCmd = &cobra.Command{
		Use:   "tool [preset_name|list]",
		Short: "List tool presets or show installation details",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTool,
	}

	rootCmd.AddCommand(assignCmd, startCmd, stopCmd, restartCmd, kaikoCmd, toolCmd)

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
	var llmBotPrompt, llmBotLoop, llmSkills string
	if llmPath != "" {
		// Resolve LLM preset
		// 1. Check if the path exists directly
		if _, err := os.Stat(llmPath); err == nil {
			lConf, err = config.LoadLLMConf(llmPath)
			if err != nil {
				return err
			}
		} else {
			// 2. Try to resolve as a preset name (directory or .toml file)
			presetsLLMDir := "presets/llms"
			if _, err := os.Stat(presetsLLMDir); os.IsNotExist(err) {
				if exePath, err := os.Executable(); err == nil {
					presetsLLMDir = filepath.Join(filepath.Dir(exePath), "presets/llms")
				}
			}
			var err error
			lConf, llmBotPrompt, llmBotLoop, llmSkills, err = config.LoadLLMPreset(presetsLLMDir, llmPath)
			if err != nil {
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

	var toolBotPrompts []string
	var toolBotLoops []string
	var toolRestartPolicy string
	var toolRestartDelay int

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
			tpData, err := config.LoadToolPreset(presetsDir, input)
			if err != nil {
				return err
			}
			list = tpData.ToolList
			if tpData.BotPrompt != "" {
				toolBotPrompts = append(toolBotPrompts, tpData.BotPrompt)
			}
			if tpData.BotLoop != "" {
				toolBotLoops = append(toolBotLoops, tpData.BotLoop)
			}
			if tpData.RestartPolicy != "" {
				toolRestartPolicy = tpData.RestartPolicy
			}
			if tpData.RestartDelay > 0 {
				toolRestartDelay = tpData.RestartDelay
			}
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
	env, err := generator.GenerateEnvWithAgentID(cfg, config.ResolveDefaultAgentID(targetDir))
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

	// Create home mount and conf directories
	renkinConfDir := filepath.Join(targetDir, ".renkin", "conf")
	if err := utils.EnsureDir(renkinConfDir); err != nil {
		return fmt.Errorf("failed to create renkin conf directory: %v", err)
	}

	if cfg.LLM != nil {
		for _, hm := range cfg.LLM.HomeMounts {
			hmDir := filepath.Join(targetDir, hm.HostDir)
			if err := utils.EnsureDir(hmDir); err != nil {
				return fmt.Errorf("failed to create home mount directory %s: %v", hmDir, err)
			}
		}
	}

	// Generate LLM configs in .renkin/conf
	geminiSettings := generator.GenerateGeminiSettings(cfg)
	if geminiSettings != "" {
		if err := os.WriteFile(filepath.Join(renkinConfDir, "settings.json"), []byte(geminiSettings), 0644); err != nil {
			return err
		}
		fmt.Println("Generated .renkin/conf/settings.json")
	}

	antigravityConfig := generator.GenerateAntigravityMCPConfig(cfg)
	if antigravityConfig != "" {
		if err := os.WriteFile(filepath.Join(renkinConfDir, "mcp_config.json"), []byte(antigravityConfig), 0644); err != nil {
			return err
		}
		fmt.Println("Generated .renkin/conf/mcp_config.json")
	}

	codexConfig := generator.GenerateCodexConfig(cfg)
	if codexConfig != "" {
		if err := os.WriteFile(filepath.Join(renkinConfDir, "config.toml"), []byte(codexConfig), 0644); err != nil {
			return err
		}
		fmt.Println("Generated .renkin/conf/config.toml")
	}

	var llmCmd string
	var loopCmd, restartPolicy, logDir, stdoutLog, stderrLog string
	var restartDelay int
	if lConf != nil {
		llmCmd = lConf.Cmd
		loopCmd = lConf.LoopCmd
		restartPolicy = lConf.RestartPolicy
		restartDelay = lConf.RestartDelay
		if toolRestartPolicy != "" {
			restartPolicy = toolRestartPolicy
		}
		if toolRestartDelay > 0 {
			restartDelay = toolRestartDelay
		}
		logDir = lConf.LogDir
		stdoutLog = lConf.StdoutLog
		stderrLog = lConf.StderrLog

		skillName, err := lConf.GetSkillFileName()
		if err != nil {
			return err
		}

		// Generate default bot_prompt.md or merge with toolBotPrompts
		botPromptPath := filepath.Join(renkinConfDir, "bot_prompt.md")
		basePrompt := llmBotPrompt
		if basePrompt == "" {
			basePrompt = `## Task
Analyze the files in your workspace. Check if there are any new files or changes, write a brief summary report of the workspace status to stdout, and print the current time.
`
		}
		var finalPrompt strings.Builder
		finalPrompt.WriteString(basePrompt)
		for _, tp := range toolBotPrompts {
			finalPrompt.WriteString("\n\n")
			finalPrompt.WriteString(tp)
		}
		if err := os.WriteFile(botPromptPath, []byte(finalPrompt.String()), 0644); err != nil {
			return err
		}
		fmt.Println("Generated .renkin/conf/bot_prompt.md")

		// Generate and write bot-loop.sh ONLY if a template exists in presets
		selectedLoopTemplate := llmBotLoop
		if len(toolBotLoops) > 0 {
			selectedLoopTemplate = toolBotLoops[0]
		}
		if selectedLoopTemplate != "" {
			finalLoop := config.RenderLoopTemplate(selectedLoopTemplate, lConf, targetDir)

			// Write to .renkin/conf/bot-loop.sh and workspace/bot-loop.sh
			if err := os.WriteFile(filepath.Join(renkinConfDir, "bot-loop.sh"), []byte(finalLoop), 0755); err != nil {
				return err
			}
			fmt.Println("Generated .renkin/conf/bot-loop.sh")

			workspaceDir := filepath.Join(targetDir, "workspace")
			if err := os.MkdirAll(workspaceDir, 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(workspaceDir, "bot-loop.sh"), []byte(finalLoop), 0755); err != nil {
				return err
			}
			fmt.Println("Generated workspace/bot-loop.sh")
		}

		var aggregatedSkills strings.Builder

		// 1. Add docker instructions
		if cfg.Docker.Instructions != "" {
			aggregatedSkills.WriteString("## Environment Instructions\n")
			aggregatedSkills.WriteString(cfg.Docker.Instructions)
			aggregatedSkills.WriteString("\n\n")
		}

		// 2. Add tool instructions
		for _, t := range cfg.ToolList.Tools {
			if t.Instructions != "" {
				aggregatedSkills.WriteString(fmt.Sprintf("## %s Instructions\n", t.Name))
				aggregatedSkills.WriteString(t.Instructions)
				aggregatedSkills.WriteString("\n\n")
			}
		}

		// 3. Add base skills from file if provided
		if skillsPath != "" {
			content, err := os.ReadFile(skillsPath)
			if err != nil {
				return err
			}
			aggregatedSkills.WriteString("## Base Skills\n")
			aggregatedSkills.Write(content)
			aggregatedSkills.WriteString("\n")
		} else if llmSkills != "" {
			aggregatedSkills.WriteString("## Base Skills\n")
			aggregatedSkills.WriteString(llmSkills)
			aggregatedSkills.WriteString("\n")
		}

		if err := os.WriteFile(filepath.Join(renkinConfDir, skillName), []byte(aggregatedSkills.String()), 0644); err != nil {
			return err
		}
		fmt.Printf("Generated .renkin/conf/%s\n", skillName)
	}

	// Save metadata
	meta := config.Metadata{
		LLMCmd:        llmCmd,
		EnvKeys:       cfg.CollectEnvKeys(),
		LoopCmd:       loopCmd,
		RestartPolicy: restartPolicy,
		RestartDelay:  restartDelay,
		LogDir:        logDir,
		StdoutLog:     stdoutLog,
		StderrLog:     stderrLog,
	}
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

	metadataPath := ".renkin_metadata.toml"
	var meta config.Metadata
	if _, err := os.Stat(metadataPath); err == nil {
		if err := config.LoadMetadata(metadataPath, &meta); err == nil {
			envFileValues := loadNonEmptyEnvFileValues(".env")
			missing := missingEnvKeys(meta.EnvKeys, os.Getenv, envFileValues)
			if len(missing) > 0 {
				fmt.Printf("Warning: The following environment variables are not set in your host environment:\n")
				for _, key := range missing {
					fmt.Printf("  - %s\n", key)
				}
				if !utils.AskForConfirmation("Do you want to continue anyway?") {
					return fmt.Errorf("aborted due to missing environment variables")
				}
			}
		}
	}

	fmt.Println("Starting containers...")
	if err := docker.ComposeUp(); err != nil {
		return err
	}

	if !noConfig {
		if err := docker.Exec("llm-agent", "if command -v renkin-generate-llm-config >/dev/null 2>&1; then renkin-generate-llm-config; fi"); err != nil {
			return err
		}
	}

	var cmdToRun string
	isLoopMode := false

	// Determine session ID and check loop flag
	if loopCmd != "" {
		isLoopMode = true

		if overrideCmd != "" {
			cmdToRun = overrideCmd
		} else if loopCmd == "default" {
			loopScriptPath := filepath.Join(".renkin", "conf", "bot-loop.sh")
			if _, err := os.Stat(loopScriptPath); os.IsNotExist(err) {
				return fmt.Errorf("loop script not found at %s. The active presets do not define a loop script. Please specify a custom loop script directly (e.g., --loop 'work.sh') or use presets that support loops", loopScriptPath)
			}
			cmdToRun = "bash /renkin-conf/bot-loop.sh"
		} else {
			cmdToRun = loopCmd
		}
	} else {
		cmdToRun = determineCommand(meta.LLMCmd, overrideCmd)
	}

	if cmdToRun != "" {
		if isLoopMode {
			return runDaemon(cmdToRun, meta)
		} else {
			fmt.Printf("Attaching to container with command: %s\n", cmdToRun)
			return docker.ExecAttach("llm-agent", cmdToRun)
		}
	}

	fmt.Println("Containers started. No LLM agent to attach.")
	return nil
}

func runDaemon(cmdToRun string, meta config.Metadata) error {
	logDir := meta.LogDir
	if logDir == "" {
		logDir = ".renkin/logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	stdoutFile := "stdout.log"
	if meta.StdoutLog != "" {
		stdoutFile = meta.StdoutLog
	}
	stderrFile := "stderr.log"
	if meta.StderrLog != "" {
		stderrFile = meta.StderrLog
	}

	stdoutPath := filepath.Join(logDir, stdoutFile)
	stderrPath := filepath.Join(logDir, stderrFile)

	restartPolicy := meta.RestartPolicy
	if restartPolicy == "" {
		restartPolicy = "always"
	}

	restartDelay := meta.RestartDelay
	if restartDelay <= 0 {
		restartDelay = 2
	}

	fmt.Printf("Starting bot loop with command: %s\n", cmdToRun)
	fmt.Printf("Restart policy: %s (delay: %ds)\n", restartPolicy, restartDelay)
	fmt.Printf("Logging stdout to: %s\n", stdoutPath)
	fmt.Printf("Logging stderr to: %s\n", stderrPath)

	lastExitCode := 0
	unknownRestartPolicy := false

	for {
		// Open log files in append mode
		outF, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		errF, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			outF.Close()
			return err
		}

		nowStr := time.Now().Format("2006-01-02 15:04:05")
		startMarker := fmt.Sprintf("\n=== SESSION START: %s (Command: %s) ===\n", nowStr, cmdToRun)
		outF.WriteString(startMarker)
		errF.WriteString(startMarker)

		// Run the command using 'docker compose exec -T'
		runCmd := exec.Command("docker", "compose", "exec", "-T", "llm-agent", "bash", "-c", cmdToRun)
		runCmd.Stdout = outF
		runCmd.Stderr = errF

		err = runCmd.Run()

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1 // System error (e.g. docker compose not running)
			}
		}
		lastExitCode = exitCode

		nowStrEnd := time.Now().Format("2006-01-02 15:04:05")
		endMarker := fmt.Sprintf("\n=== SESSION END: %s (Exit Code: %d) ===\n", nowStrEnd, exitCode)
		outF.WriteString(endMarker)
		errF.WriteString(endMarker)

		outF.Close()
		errF.Close()

		fmt.Printf("Loop iteration ended at %s with exit code %d\n", nowStrEnd, exitCode)

		// Check restart policy
		shouldRestart, knownPolicy := shouldRestartLoop(restartPolicy, exitCode)
		if !knownPolicy {
			fmt.Printf("Unknown restart policy %q. Stopping loop.\n", restartPolicy)
			unknownRestartPolicy = true
		}

		if !shouldRestart {
			break
		}

		fmt.Printf("Restart policy triggered. Sleeping %d seconds before restarting...\n", restartDelay)
		time.Sleep(time.Duration(restartDelay) * time.Second)
	}

	if unknownRestartPolicy {
		return fmt.Errorf("unknown restart policy %q", restartPolicy)
	}
	if lastExitCode != 0 {
		return fmt.Errorf("loop command exited with code %d", lastExitCode)
	}

	return nil
}

func shouldRestartLoop(restartPolicy string, exitCode int) (bool, bool) {
	switch restartPolicy {
	case "", "never":
		return false, true
	case "always":
		return true, true
	case "on-failure":
		return exitCode != 0, true
	case "on-success":
		return exitCode == 0, true
	default:
		return false, false
	}
}

func missingEnvKeys(keys []string, getenv func(string) string, envFileValues map[string]string) []string {
	var missing []string
	for _, key := range keys {
		if getenv(key) == "" && envFileValues[key] == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func loadNonEmptyEnvFileValues(path string) map[string]string {
	values := make(map[string]string)
	file, err := os.Open(path)
	if err != nil {
		return values
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		values[key] = trimEnvQuotes(value)
	}
	return values
}

func trimEnvQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func determineCommand(metaLLMCmd, overrideCmd string) string {
	if overrideCmd != "" {
		return overrideCmd
	}
	return metaLLMCmd
}

func runStop(cmd *cobra.Command, args []string) error {
	fmt.Println("Stopping containers and removing volumes...")
	return docker.ComposeDown()
}

func runRestart(cmd *cobra.Command, args []string) error {
	rebuild, _ := cmd.Flags().GetBool("rebuild")

	fmt.Println("Restarting environment...")
	if err := docker.ComposeDown(); err != nil {
		return err
	}

	if rebuild {
		fmt.Println("Rebuilding images (no-cache)...")
		if err := docker.ComposeBuild(true); err != nil {
			return err
		}
	}

	return runStart(cmd, args)
}

func runKaiko(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("yes")
	if !force && !utils.AskForConfirmation("This will completely remove all containers, images, and volumes for this project. Are you sure?") {
		fmt.Println("Aborted.")
		return nil
	}
	fmt.Println("Executing Kaiko (thorough cleanup)...")
	return docker.ComposeKaiko()
}

func runTool(cmd *cobra.Command, args []string) error {
	presetsDir := "presets/tools"
	if _, err := os.Stat(presetsDir); os.IsNotExist(err) {
		if exePath, err := os.Executable(); err == nil {
			presetsDir = filepath.Join(filepath.Dir(exePath), "presets/tools")
		}
	}

	if len(args) == 0 || args[0] == "list" {
		files, err := os.ReadDir(presetsDir)
		if err != nil {
			return fmt.Errorf("failed to read presets directory: %v", err)
		}

		fmt.Println("Available tool presets:")
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".toml" {
				fmt.Printf("  - %s\n", strings.TrimSuffix(f.Name(), ".toml"))
			}
		}
		return nil
	}

	presetName := args[0]
	tList, err := config.LoadToolList(filepath.Join(presetsDir, presetName+".toml"))
	if err != nil {
		return fmt.Errorf("failed to load preset %s: %v", presetName, err)
	}

	fmt.Printf("Details for tool preset: %s\n", presetName)
	for _, t := range tList.Tools {
		fmt.Printf("\n--- Tool: %s ---\n", t.Name)
		if t.Preset != "" {
			fmt.Printf("Base Preset: %s\n", t.Preset)
		}
		fmt.Printf("Type: %s\n", t.Type)
		if t.Install != "" {
			fmt.Printf("Installation:\n%s\n", t.Install)
		}
		if t.Instructions != "" {
			fmt.Printf("Instructions:\n%s\n", t.Instructions)
		}
		if len(t.Environment) > 0 {
			fmt.Printf("Environment Variables: %v\n", t.Environment)
		}
	}

	return nil
}
