package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiToolSkillSynthesis(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-multi-skill-test")
	defer os.RemoveAll(tmpDir)
	
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Prepare presets
	presetsDir := filepath.Join(tmpDir, "presets", "tools")
	os.MkdirAll(presetsDir, 0755)
	
	tool1 := `[[tool]]
name = "tool1"
type = "shell"
install = "RUN echo tool1"
instructions = "Instructions for tool1"
`
	tool2 := `[[tool]]
name = "tool2"
type = "shell"
install = "RUN echo tool2"
instructions = "Instructions for tool2"
`
	os.WriteFile(filepath.Join(presetsDir, "tool1.toml"), []byte(tool1), 0644)
	os.WriteFile(filepath.Join(presetsDir, "tool2.toml"), []byte(tool2), 0644)

	dockerConf := `base_image = "ubuntu:24.04"`
	llmConf := `cmd = "gemini"
install = "RUN echo llm"
`
	toolList := `
[[tool]]
preset = "tool1"
[[tool]]
preset = "tool2"
`
	skills := "# Base User Skills"

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "llm.conf"), []byte(llmConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(toolList), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "skills.md"), []byte(skills), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--llm", filepath.Join(fixtureDir, "llm.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
		"--skills", filepath.Join(fixtureDir, "skills.md"),
	)
	assignCmd.Dir = tmpDir
	output, err := assignCmd.CombinedOutput()
	assert.NoError(t, err, string(output))

	// Verify synthesized skill file
	skillFile := filepath.Join(targetDir, "workspace", "GEMINI.md")
	assert.FileExists(t, skillFile)

	content, err := os.ReadFile(skillFile)
	assert.NoError(t, err)
	
	s := string(content)
	assert.Contains(t, s, "## tool1 Instructions")
	assert.Contains(t, s, "Instructions for tool1")
	assert.Contains(t, s, "## tool2 Instructions")
	assert.Contains(t, s, "Instructions for tool2")
	assert.Contains(t, s, "## Base Skills")
	assert.Contains(t, s, "# Base User Skills")
}

func TestSkillSynthesisWithoutBaseSkills(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "renkin-no-base-skill-test")
	defer os.RemoveAll(tmpDir)
	
	binPath := filepath.Join(tmpDir, "renkin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/renkin")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build renkin: %v", err)
	}

	// Prepare presets
	presetsDir := filepath.Join(tmpDir, "presets", "tools")
	os.MkdirAll(presetsDir, 0755)
	
	tool1 := `[[tool]]
name = "tool1"
type = "shell"
install = "RUN echo tool1"
instructions = "Instructions for tool1"
`
	os.WriteFile(filepath.Join(presetsDir, "tool1.toml"), []byte(tool1), 0644)

	dockerConf := `base_image = "ubuntu:24.04"`
	llmConf := `cmd = "gemini"
install = "RUN echo llm"
`
	toolList := `
[[tool]]
preset = "tool1"
`

	fixtureDir := filepath.Join(tmpDir, "fixtures")
	os.MkdirAll(fixtureDir, 0755)
	os.WriteFile(filepath.Join(fixtureDir, "docker.conf"), []byte(dockerConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "llm.conf"), []byte(llmConf), 0644)
	os.WriteFile(filepath.Join(fixtureDir, "tool_list.toml"), []byte(toolList), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	assignCmd := exec.Command(binPath, "assign", targetDir,
		"--docker", filepath.Join(fixtureDir, "docker.conf"),
		"--llm", filepath.Join(fixtureDir, "llm.conf"),
		"--tools", filepath.Join(fixtureDir, "tool_list.toml"),
		// No --skills flag, and no skills.md in targetDir
	)
	assignCmd.Dir = tmpDir
	output, err := assignCmd.CombinedOutput()
	assert.NoError(t, err, string(output))

	// Verify synthesized skill file exists and contains tool instructions
	skillFile := filepath.Join(targetDir, "workspace", "GEMINI.md")
	assert.FileExists(t, skillFile)

	content, err := os.ReadFile(skillFile)
	assert.NoError(t, err)
	
	s := string(content)
	assert.Contains(t, s, "## tool1 Instructions")
	assert.Contains(t, s, "Instructions for tool1")
	assert.NotContains(t, s, "## Base Skills")
}
