package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type fakeRegistrySource struct {
	login *entities.RegistryLogin
	err   error
}

func (f fakeRegistrySource) RegistryLogin(context.Context) (*entities.RegistryLogin, error) {
	return f.login, f.err
}

type fakeDocker struct{ server, user, pass string }

func (f *fakeDocker) DockerLogin(_ context.Context, server, user, pass string) error {
	f.server, f.user, f.pass = server, user, pass
	return nil
}

func TestRegistryLoginPassesCredentialToDocker(t *testing.T) {
	docker := &fakeDocker{}
	login := &entities.RegistryLogin{Server: "registry.test", Username: "paas", Password: "secret", Repository: "registry.test/o/s"}
	got, err := NewRegistryLogin(fakeRegistrySource{login: login}, docker).Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Repository != "registry.test/o/s" || docker.server != "registry.test" || docker.user != "paas" || docker.pass != "secret" {
		t.Fatalf("got %+v, docker %+v", got, docker)
	}
}

func TestRegistryLoginStopsWhenPlatformRefuses(t *testing.T) {
	docker := &fakeDocker{}
	_, err := NewRegistryLogin(fakeRegistrySource{err: errors.New("403")}, docker).Execute(context.Background())
	if err == nil || docker.server != "" {
		t.Fatalf("err %v, docker called %+v", err, docker)
	}
}
