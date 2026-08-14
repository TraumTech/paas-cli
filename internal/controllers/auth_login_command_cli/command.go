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

// passwordFlag переводит вход на прежний путь — ввод пароля в терминал. Он нужен
// там, где браузера рядом нет (машина по ssh), поэтому остаётся явной отдушиной,
// а не умолчанием: пароль лучше вводить в форму платформы, а не в чужой процесс.
const passwordFlag = "password"

type Command struct {
	login   UserLogin
	browser BrowserLogin
}

func New(login UserLogin, browser BrowserLogin) *Command {
	return &Command{login: login, browser: browser}
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
				Usage:   "e-mail учётной записи (без флага будет запрошен); подразумевает --password",
			},
			&cli.BoolFlag{
				Name:  passwordFlag,
				Usage: "войти паролем в терминале вместо подтверждения в браузере",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	out := cmd.Root().Writer
	in := bufio.NewReader(cmd.Root().Reader)

	email := cmd.String(emailFlag)
	// По умолчанию вход идёт через браузер; заданный e-mail — явное намерение
	// войти паролем, отдельного флага для этого требовать незачем.
	if !cmd.Bool(passwordFlag) && email == "" {
		return c.runBrowser(ctx, out)
	}
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

// runBrowser ведёт вход через браузер: подтверждение и выдача личного токена
// происходят в интерфейсе платформы, CLI получает готовый токен сам.
func (c *Command) runBrowser(ctx context.Context, out io.Writer) error {
	result, err := c.browser.Execute(ctx)
	if err != nil {
		return err
	}
	whom := result.Email
	if whom == "" {
		whom = "владелец токена"
	}
	fmt.Fprintf(out, "✓ Вы вошли как %s — вход сохранён, действует до %s\n",
		whom, result.ExpiresAt.Local().Format("02.01.2006"))
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
