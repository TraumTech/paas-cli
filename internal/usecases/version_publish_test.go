package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func nameManifest() *entities.Manifest {
	return &entities.Manifest{Service: &entities.ManifestService{Name: "paas-backend"}}
}

type publishFixture struct {
	manifests *MockManifestReader
	forms     *MockFormReader
	resolver  *MockServiceResolver
	publisher *MockVersionPublisher
	uc        *PublishVersionUseCase
}

func newPublishFixture(t *testing.T) *publishFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &publishFixture{
		manifests: NewMockManifestReader(ctrl),
		forms:     NewMockFormReader(ctrl),
		resolver:  NewMockServiceResolver(ctrl),
		publisher: NewMockVersionPublisher(ctrl),
	}
	f.uc = NewPublishVersion(f.manifests, f.forms, f.resolver, f.publisher)
	return f
}

func (f *publishFixture) input() PublishVersionInput {
	return PublishVersionInput{CommitRevision: "abc123", Environment: "prod", ManifestPath: "protocols.toml", FormPath: "paas.toml"}
}

func (f *publishFixture) expectResolved() {
	f.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
}

func TestPublishVersionExecute_Success(t *testing.T) {
	f := newPublishFixture(t)
	version := &entities.Version{ID: "ver-1", Number: 7, CommitRevision: "abc123"}

	f.expectResolved()
	// paas.toml отсутствует — публикация без формы, как раньше.
	f.forms.EXPECT().Read(gomock.Any(), "paas.toml").Return(nil, nil)
	f.publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "prod", "abc123", "", nil).Return(version, nil)

	got, err := f.uc.Execute(context.Background(), f.input())

	require.NoError(t, err)
	assert.Same(t, version, got)
}

// Форма из paas.toml едет с версией вместе с адресом образа (DEP-02).
func TestPublishVersionExecute_WithForm(t *testing.T) {
	f := newPublishFixture(t)
	declaration := &entities.FormDeclaration{Processes: []entities.ProcessForm{{Name: "server", Listen: 8080}}}
	version := &entities.Version{ID: "ver-1", Number: 8, CommitRevision: "abc123"}

	f.expectResolved()
	f.forms.EXPECT().Read(gomock.Any(), "paas.toml").Return(declaration, nil)
	f.publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "prod", "abc123", "ghcr.io/traumtech/svc:sha",
		declaration.Resolve("prod")).Return(version, nil)

	in := f.input()
	in.Image = "ghcr.io/traumtech/svc:sha"
	_, err := f.uc.Execute(context.Background(), in)

	require.NoError(t, err)
}

// Форма без образа не имеет смысла — отказ до обращения к сети.
func TestPublishVersionExecute_FormRequiresImage(t *testing.T) {
	f := newPublishFixture(t)

	f.expectResolved()
	f.forms.EXPECT().Read(gomock.Any(), "paas.toml").Return(&entities.FormDeclaration{}, nil)

	_, err := f.uc.Execute(context.Background(), f.input())

	assert.ErrorIs(t, err, entities.ErrFormRequiresImage)
}

func TestPublishVersionExecute_EmptyRevision_NoManifest(t *testing.T) {
	f := newPublishFixture(t)
	// Манифест/резолвер/публикацию не трогаем — без ревизии останавливаемся сразу.
	in := f.input()
	in.CommitRevision = "  "

	_, err := f.uc.Execute(context.Background(), in)

	assert.ErrorIs(t, err, entities.ErrEmptyCommitRevision)
}

func TestPublishVersionExecute_NoServiceDeclared_NoPublish(t *testing.T) {
	f := newPublishFixture(t)
	f.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(&entities.Manifest{}, nil)
	// резолвер/публикация не вызываются — манифест не объявляет сервис.

	_, err := f.uc.Execute(context.Background(), f.input())

	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestPublishVersionExecute_ServiceNotFound_NoPublish(t *testing.T) {
	f := newPublishFixture(t)
	f.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{}, nil)

	_, err := f.uc.Execute(context.Background(), f.input())

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestPublishVersionExecute_SourceError(t *testing.T) {
	f := newPublishFixture(t)
	srcErr := errors.New("boom")

	f.expectResolved()
	f.forms.EXPECT().Read(gomock.Any(), "paas.toml").Return(nil, nil)
	f.publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "prod", "abc123", "", nil).Return(nil, srcErr)

	_, err := f.uc.Execute(context.Background(), f.input())

	assert.ErrorIs(t, err, srcErr)
}

// Неизвестное окружение отсекается до чтения манифеста и сети (DEP-08).
func TestPublishVersionExecute_UnknownEnvironment(t *testing.T) {
	f := newPublishFixture(t)
	in := f.input()
	in.Environment = "qa"

	_, err := f.uc.Execute(context.Background(), in)

	assert.ErrorIs(t, err, entities.ErrUnknownEnvironment)
}

// Значения окружений разрешаются при публикации: версия принадлежит окружению,
// поэтому платформа получает готовый набор, а не правило слияния (DEP-14/15).
func TestPublishVersionExecute_ResolvesEnvironmentValues(t *testing.T) {
	f := newPublishFixture(t)
	declaration := &entities.FormDeclaration{
		Processes: []entities.ProcessForm{{Name: "server", Listen: 8080}},
		Environments: map[string]entities.EnvironmentValues{
			entities.DefaultEnvironmentKey: {
				Variables: map[string]string{"LOG_LEVEL": "info", "REGION": "ru"},
				Replicas:  1,
			},
			"prod": {
				Variables: map[string]string{"LOG_LEVEL": "warn"},
				Replicas:  2,
			},
		},
	}
	version := &entities.Version{ID: "ver-1", Number: 9, CommitRevision: "abc123"}

	f.expectResolved()
	f.forms.EXPECT().Read(gomock.Any(), "paas.toml").Return(declaration, nil)
	f.publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "prod", "abc123", "img", gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _, _ string, form *entities.VersionForm) (*entities.Version, error) {
			// Переопределение перебивает одноимённое общее значение, остальные
			// общие остаются; порядок детерминирован.
			assert.Equal(t, []entities.FormVariable{
				{Name: "LOG_LEVEL", Value: "warn"},
				{Name: "REGION", Value: "ru"},
			}, form.Variables)
			assert.Equal(t, 2, form.Replicas)
			return version, nil
		})

	in := f.input()
	in.Image = "img"
	_, err := f.uc.Execute(context.Background(), in)

	require.NoError(t, err)
}

// Секция [env.<имя>] для окружения, которого у платформы нет, — опечатка;
// молча никуда не доехать она не должна.
func TestPublishVersionExecute_UnknownFormEnvironment(t *testing.T) {
	f := newPublishFixture(t)

	f.expectResolved()
	f.forms.EXPECT().Read(gomock.Any(), "paas.toml").Return(&entities.FormDeclaration{
		Processes:    []entities.ProcessForm{{Name: "server"}},
		Environments: map[string]entities.EnvironmentValues{"prd": {Replicas: 2}},
	}, nil)

	in := f.input()
	in.Image = "img"
	_, err := f.uc.Execute(context.Background(), in)

	assert.ErrorIs(t, err, entities.ErrUnknownFormEnvironment)
}
