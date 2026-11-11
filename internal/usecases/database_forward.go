package usecases

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type ForwardDatabaseInput struct {
	Server string
	// Namespace — где искать СУБД; пусто — во всём кластере.
	Namespace string
	// Role — личная роль владельца; заводится, если её ещё нет.
	Role string
	Port int
	// Context и Kubeconfig — доступ владельца к кластеру; пусто — обычное
	// разрешение kubeconfig.
	Context    string
	Kubeconfig string
}

// ForwardDatabasePlan — что команда изменит в кластере, если роли ещё нет.
type ForwardDatabasePlan struct {
	ContextName string
	Server      entities.DatabaseServer
	Role        string
}

type ForwardDatabaseConfirmFunc func(plan ForwardDatabasePlan) (bool, error)

// ForwardDatabaseSession — данные для клиента БД, когда проброс поднят.
type ForwardDatabaseSession struct {
	Server      entities.DatabaseServer
	Port        int
	Credentials entities.DatabaseCredentials
}

// ForwardDatabaseEvents — что показать владельцу по ходу работы.
type ForwardDatabaseEvents struct {
	Ready       func(ForwardDatabaseSession)
	Lost        func(err error, retryIn time.Duration)
	Reconnected func()
}

const (
	forwardBackoffMin = time.Second
	forwardBackoffMax = 30 * time.Second
	// forwardStableAfter — сессия, прожившая столько, считается удачной:
	// следующий обрыв снова начинается с короткой паузы.
	forwardStableAfter = time.Minute
)

type ForwardDatabaseUseCase struct {
	access DatabaseAccess
	sleep  func(ctx context.Context, d time.Duration) error
	now    func() time.Time
}

func NewForwardDatabase(access DatabaseAccess) *ForwardDatabaseUseCase {
	return &ForwardDatabaseUseCase{access: access, sleep: sleepContext, now: time.Now}
}

// Execute держит локальный порт на основном инстансе СУБД, пока не отменён
// ctx: обрыв (рестарт пода, сон ноутбука, таймаут kubelet) не завершает
// команду, а переподключает проброс.
func (uc *ForwardDatabaseUseCase) Execute(
	ctx context.Context,
	input ForwardDatabaseInput,
	confirm ForwardDatabaseConfirmFunc,
	events ForwardDatabaseEvents,
) error {
	name := strings.TrimSpace(input.Server)
	if name == "" {
		return entities.ErrEmptyDatabaseServer
	}
	if err := entities.ValidatePersonalRole(input.Role); err != nil {
		return err
	}
	if input.Port < 1 || input.Port > 65535 {
		return entities.ErrInvalidLocalPort
	}

	cluster, err := uc.access.Connect(input.Kubeconfig, input.Context)
	if err != nil {
		return err
	}
	server, err := cluster.Find(ctx, name, strings.TrimSpace(input.Namespace))
	if err != nil {
		return err
	}

	if !server.HasRole(input.Role) {
		agreed, err := confirm(ForwardDatabasePlan{ContextName: cluster.ContextName(), Server: *server, Role: input.Role})
		if err != nil {
			return err
		}
		if !agreed {
			return entities.ErrCancelled
		}
		if err := cluster.EnsureRole(ctx, *server, input.Role); err != nil {
			return err
		}
	}
	creds, err := cluster.Credentials(ctx, *server, input.Role)
	if err != nil {
		return err
	}

	return uc.forward(ctx, cluster, *server, input.Port, ForwardDatabaseSession{Server: *server, Port: input.Port, Credentials: *creds}, events)
}

func (uc *ForwardDatabaseUseCase) forward(
	ctx context.Context,
	cluster DatabaseCluster,
	server entities.DatabaseServer,
	port int,
	session ForwardDatabaseSession,
	events ForwardDatabaseEvents,
) error {
	backoff := forwardBackoffMin
	announced, lost := false, false
	for {
		started := uc.now()
		err := cluster.Forward(ctx, server, port, func() {
			switch {
			case !announced:
				announced = true
				events.Ready(session)
			case lost:
				events.Reconnected()
			}
			lost = false
		})
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, entities.ErrLocalPortBusy) || errors.Is(err, entities.ErrDatabaseAccessDenied) {
			return err
		}
		if uc.now().Sub(started) >= forwardStableAfter {
			backoff = forwardBackoffMin
		}
		lost = true
		events.Lost(err, backoff)
		if uc.sleep(ctx, backoff) != nil {
			return nil
		}
		backoff = min(backoff*2, forwardBackoffMax)
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
