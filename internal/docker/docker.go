package docker

import (
	"os"
	"os/exec"
)

func ComposeUp() error {
	cmd := exec.Command("docker", "compose", "up", "-d")
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

func ExecAttach(serviceName string, command string) error {
	// Use 'docker compose exec' to target the service name directly.
	// It automatically handles project-prefixed container names.
	cmd := exec.Command("docker", "compose", "exec", serviceName, "bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
