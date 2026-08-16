// Package buildpublishcommandcli — входной адаптер: подкоманда `builds publish`
// (DEP-18). Публикует артефакт сборки: ревизию, ветку, образ, форму со всеми
// секциями окружений и контракт. Окружение не называется — его выбирает выкатка.
package buildpublishcommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	manifestFlag = "manifest"
	formFlag     = "form"
	imageFlag    = "image"
	branchFlag   = "branch"
)

const (
	// Пустой путь — выбор за читателем: paas.toml, при его отсутствии
	// переходный protocols.toml (CLI-22).
	defaultManifestPath = ""
	defaultFormPath     = "paas.toml"
)

type Command struct {
	publisher BuildPublisher
}

func New(publisher BuildPublisher) *Command {
	return &Command{publisher: publisher}
}

func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "publish",
		Usage:     "опубликовать сборку ветки (сервис и контракт — из манифеста)",
		ArgsUsage: "<commit-revision>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    manifestFlag,
				Aliases: []string{"f"},
				Value:   defaultManifestPath,
				Usage:   "путь к манифесту (по умолчанию paas.toml, при его отсутствии protocols.toml)",
			},
			&cli.StringFlag{
				Name:  formFlag,
				Value: defaultFormPath,
				Usage: "путь к форме сервиса; файла нет — сборка публикуется без формы",
			},
			&cli.StringFlag{
				Name:  imageFlag,
				Usage: "адрес образа этой сборки; обязателен вместе с формой",
			},
			&cli.StringFlag{
				Name:  branchFlag,
				Usage: "ветка, из которой собран артефакт (в CI — ref сборки)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("нужно указать <commit-revision> (имя сервиса берётся из манифеста)")
	}

	build, err := c.publisher.Execute(ctx, usecases.PublishBuildInput{
		CommitRevision: cmd.Args().Get(0),
		Branch:         cmd.String(branchFlag),
		ManifestPath:   cmd.String(manifestFlag),
		FormPath:       cmd.String(formFlag),
		Image:          cmd.String(imageFlag),
	})
	if err != nil {
		return err
	}

	// Идентификатор сборки — на stdout отдельной строкой: его подхватывает
	// следующий шаг (выкатка), как раньше подхватывал id версии.
	fmt.Fprintln(cmd.Root().Writer, build.ID)
	fmt.Fprintf(cmd.Root().ErrWriter, "✓ Сборка ревизии %s (%s) зафиксирована\n",
		build.CommitRevision, branchLabel(build.Branch))
	return nil
}

// branchLabel — ветка для человека; пустая означает, что её не сообщили.
func branchLabel(branch string) string {
	if branch == "" {
		return "ветка не указана"
	}
	return "ветка " + branch
}
