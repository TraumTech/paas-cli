// Package dockerloginexec передаёт вход в реестр локальному docker: пароль —
// через stdin, чтобы он не попал в список процессов и логи CI.
package dockerloginexec

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Login struct {
	output io.Writer
}

// output — куда уходит вывод docker (stderr: stdout команды — адрес репозитория).
func New(output io.Writer) *Login {
	return &Login{output: output}
}

func (l *Login) DockerLogin(ctx context.Context, server, username, password string) error {
	cmd := exec.CommandContext(ctx, "docker", "login", server, "--username", username, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)
	cmd.Stdout = l.output
	cmd.Stderr = l.output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker login %s: %w", server, err)
	}
	return nil
}
