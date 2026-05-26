package docker

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

func ComposeUp() error {
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Env = composeEnv(os.Environ(), ".env")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ComposeDown() error {
	cmd := exec.Command("docker", "compose", "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Exec(serviceName string, command string) error {
	cmd := exec.Command("docker", "compose", "exec", "-T", serviceName, "bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func composeEnv(base []string, envPath string) []string {
	values, err := loadNonEmptyEnvFile(envPath)
	if err != nil || len(values) == 0 {
		return base
	}

	merged := append([]string{}, base...)
	indexByKey := make(map[string]int)
	for i, entry := range merged {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			indexByKey[key] = i
		}
	}

	for key, value := range values {
		entry := key + "=" + value
		if index, ok := indexByKey[key]; ok {
			merged[index] = entry
		} else {
			indexByKey[key] = len(merged)
			merged = append(merged, entry)
		}
	}
	return merged
}

func loadNonEmptyEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
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
	return values, scanner.Err()
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

func ExecAttach(serviceName string, command string) error {
	// Use 'docker compose exec' to target the service name directly.
	// It automatically handles project-prefixed container names.
	cmd := exec.Command("docker", "compose", "exec", serviceName, "bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
