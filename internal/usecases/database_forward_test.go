package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type fakeDatabaseCluster struct {
	server      entities.DatabaseServer
	ensured     []string
	sessions    []error
	forwardSeen int
	cancel      context.CancelFunc
}

func (f *fakeDatabaseCluster) Connect(string, string) (DatabaseCluster, error) { return f, nil }
func (f *fakeDatabaseCluster) ContextName() string                            { return "apps-ru-7" }

func (f *fakeDatabaseCluster) Find(_ context.Context, name, _ string) (*entities.DatabaseServer, error) {
	if name != f.server.Name {
		return nil, entities.ErrDatabaseServerNotFound
	}
	s := f.server
	return &s, nil
}

func (f *fakeDatabaseCluster) EnsureRole(_ context.Context, _ entities.DatabaseServer, role string) error {
	f.ensured = append(f.ensured, role)
	return nil
}

func (f *fakeDatabaseCluster) Credentials(_ context.Context, _ entities.DatabaseServer, role string) (*entities.DatabaseCredentials, error) {
	return &entities.DatabaseCredentials{User: role, Password: "secret"}, nil
}

// Forward отыгрывает сеансы по очереди: nil — порт поднят и связь потеряна,
// ошибка — сеанс не поднялся. Когда сеансы кончаются, команду отменяют.
func (f *fakeDatabaseCluster) Forward(_ context.Context, _ entities.DatabaseServer, _ int, ready func()) error {
	if f.forwardSeen >= len(f.sessions) {
		f.cancel()
		return errors.New("context canceled")
	}
	err := f.sessions[f.forwardSeen]
	f.forwardSeen++
	if err == nil {
		ready()
		return errors.New("lost connection to pod")
	}
	return err
}

type forwardLog struct {
	ready, reconnected int
	pauses             []time.Duration
}

func (l *forwardLog) events() ForwardDatabaseEvents {
	return ForwardDatabaseEvents{
		Ready:       func(ForwardDatabaseSession) { l.ready++ },
		Lost:        func(_ error, d time.Duration) { l.pauses = append(l.pauses, d) },
		Reconnected: func() { l.reconnected++ },
	}
}

func newForwardFixture(sessions ...error) (*ForwardDatabaseUseCase, *fakeDatabaseCluster, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeDatabaseCluster{server: entities.DatabaseServer{Namespace: "d0123", Name: "main"}, sessions: sessions, cancel: cancel}
	uc := NewForwardDatabase(f)
	uc.sleep = func(context.Context, time.Duration) error { return nil }
	return uc, f, ctx
}

var forwardInput = ForwardDatabaseInput{Server: "main", Role: "ruslan", Port: 15432}

func agreeForward(ForwardDatabasePlan) (bool, error) { return true, nil }

func TestForwardDatabase_CreatesMissingRoleAfterConsent(t *testing.T) {
	uc, f, ctx := newForwardFixture(nil)
	var log forwardLog

	require.NoError(t, uc.Execute(ctx, forwardInput, agreeForward, log.events()))

	assert.Equal(t, []string{"ruslan"}, f.ensured)
	assert.Equal(t, 1, log.ready)
}

func TestForwardDatabase_KeepsExistingRoleWithoutAsking(t *testing.T) {
	uc, f, ctx := newForwardFixture(nil)
	f.server.Roles = []string{"ruslan"}
	var log forwardLog

	err := uc.Execute(ctx, forwardInput, func(ForwardDatabasePlan) (bool, error) {
		t.Fatal("роль уже есть — спрашивать не о чем")
		return false, nil
	}, log.events())

	require.NoError(t, err)
	assert.Empty(t, f.ensured)
}

func TestForwardDatabase_RefusalChangesNothing(t *testing.T) {
	uc, f, ctx := newForwardFixture(nil)

	err := uc.Execute(ctx, forwardInput, func(ForwardDatabasePlan) (bool, error) { return false, nil }, (&forwardLog{}).events())

	require.ErrorIs(t, err, entities.ErrCancelled)
	assert.Empty(t, f.ensured)
	assert.Zero(t, f.forwardSeen)
}

func TestForwardDatabase_ReconnectsWithGrowingPause(t *testing.T) {
	down := errors.New("pod not running")
	uc, f, ctx := newForwardFixture(nil, down, down, nil)
	var log forwardLog

	require.NoError(t, uc.Execute(ctx, forwardInput, agreeForward, log.events()))

	assert.Equal(t, 4, f.forwardSeen)
	assert.Equal(t, 1, log.ready, "данные для клиента печатаются один раз")
	assert.Equal(t, 1, log.reconnected)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}, log.pauses)
}

func TestForwardDatabase_BusyPortStopsAtOnce(t *testing.T) {
	uc, f, ctx := newForwardFixture(entities.ErrLocalPortBusy)

	err := uc.Execute(ctx, forwardInput, agreeForward, (&forwardLog{}).events())

	require.ErrorIs(t, err, entities.ErrLocalPortBusy)
	assert.Equal(t, 1, f.forwardSeen)
}

func TestForwardDatabase_ValidatesInput(t *testing.T) {
	uc, _, ctx := newForwardFixture()
	cases := map[string]struct {
		input ForwardDatabaseInput
		want  error
	}{
		"нет СУБД":        {ForwardDatabaseInput{Role: "ruslan", Port: 1}, entities.ErrEmptyDatabaseServer},
		"занятая роль":    {ForwardDatabaseInput{Server: "main", Role: "postgres", Port: 1}, entities.ErrInvalidPersonalRole},
		"порт вне границ": {ForwardDatabaseInput{Server: "main", Role: "ruslan", Port: 70000}, entities.ErrInvalidLocalPort},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, uc.Execute(ctx, tc.input, agreeForward, (&forwardLog{}).events()), tc.want)
		})
	}
}
