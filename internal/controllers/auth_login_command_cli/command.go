package authlogincommandcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

// emailFlag — локальный флаг с e-mail учётной записи; без него e-mail запрашивается.
const emailFlag = "email"

type Command struct {
	login UserLogin
}

func New(login UserLogin) *Command {
	return &Command{login: login}
}

// CLICommand описывает подкоманду `login` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "login",
		Usage: "войти под своей учётной записью платформы",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    emailFlag,
				Aliases: []string{"e"},
				Usage:   "e-mail учётной записи (без флага будет запрошен)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	out := cmd.Root().Writer
	in := bufio.NewReader(cmd.Root().Reader)

	email := cmd.String(emailFlag)
	if email == "" {
		fmt.Fprint(out, "E-mail: ")
		line, err := readLine(in)
		if err != nil {
			return fmt.Errorf("чтение e-mail: %w", err)
		}
		email = line
	}

	fmt.Fprint(out, "Пароль: ")
	password, err := readPassword(cmd.Root().Reader, in)
	if err != nil {
		return fmt.Errorf("чтение пароля: %w", err)
	}
	fmt.Fprintln(out)

	result, err := c.login.Execute(ctx, usecases.LoginInput{
		Email:    strings.TrimSpace(email),
		Password: password,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ Вы вошли как %s — вход сохранён, команды выполняются от вашего имени\n", result.Email)
	return nil
}

// readPassword читает пароль без эха, когда ввод — настоящий терминал; иначе
// (пайп, тест) — строкой из общего ридера команды. Пароль через флаг или аргумент
// сознательно не принимаем, чтобы секрет не оседал в истории shell.
func readPassword(reader io.Reader, in *bufio.Reader) (string, error) {
	if f, ok := reader.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		raw, err := term.ReadPassword(int(f.Fd()))
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	return readLine(in)
}

func readLine(in *bufio.Reader) (string, error) {
	line, err := in.ReadString('\n')
	// Последняя строка ввода без перевода строки — тоже валидный ввод.
	if err == io.EOF && line != "" {
		err = nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
