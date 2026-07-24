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

const validDoc = `{"openapi":"3.1.0","paths":{"/x":{}}}`

const partialDoc = `{"openapi":"3.1.0","paths":{` +
	`"/a":{"get":{"operationId":"op-a","responses":{"200":{"description":"ok"}}}}}}`

func TestFetchProtocolExecute_PartialPassesMethodsToPlatform(t *testing.T) {
	// Сужение выполняет платформа: methods уходят в источник, к себе приходит и
	// сохраняется уже частичный контракт (CLI-09).
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	partial := &entities.Protocol{ServiceID: "svc", ServiceName: "svc-name", Document: []byte(partialDoc)}
	source.EXPECT().FetchProtocol(gomock.Any(), "svc", []string{"GET /a"}).Return(partial, false, nil)
	store.EXPECT().Save(gomock.Any(), partial, "protocols").Return("protocols/svc-name/openapi.json", nil)

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols", Methods: []string{"GET /a"}})
	require.NoError(t, err)
}

func TestFetchProtocolExecute_PlatformRejectsMethods_NoSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	rejected := errors.New("платформа отклонила запрос контракта: methods not found in protocol: GET /x")
	source.EXPECT().FetchProtocol(gomock.Any(), "svc", []string{"GET /x"}).Return(nil, false, rejected)
	// store.Save не вызывается — несуществующий метод не даёт записать неполный срез.

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols", Methods: []string{"GET /x"}})

	assert.ErrorIs(t, err, rejected)
}

func TestFetchProtocolExecute_NarrowingSkipped_ErrorsNoSave(t *testing.T) {
	// Явный fetch -m по контракту без поддержки сужения (gRPC) — ошибка, как и
	// раньше: пользователь просил срез, целиком молча не кладём.
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	whole := &entities.Protocol{ServiceID: "svc", Format: entities.ProtocolFormatGRPC, Document: []byte(`syntax = "proto3";`)}
	source.EXPECT().FetchProtocol(gomock.Any(), "svc", []string{"pkg.Svc/Method"}).Return(whole, true, nil)

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols", Methods: []string{"pkg.Svc/Method"}})

	assert.ErrorIs(t, err, entities.ErrMethodsUnsupportedForFormat)
}

func TestFetchProtocolExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	protocol := &entities.Protocol{ServiceID: "svc", ServiceName: "svc-name", VersionNumber: 3, Document: []byte(validDoc)}
	source.EXPECT().FetchProtocol(gomock.Any(), "svc", gomock.Nil()).Return(protocol, false, nil)
	store.EXPECT().Save(gomock.Any(), protocol, "protocols").Return("protocols/svc-name/openapi.json", nil)

	got, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols"})

	require.NoError(t, err)
	assert.Equal(t, "svc-name", got.ServiceName)
	assert.Equal(t, 3, got.VersionNumber)
	assert.Equal(t, "protocols/svc-name/openapi.json", got.Path)
}

func TestFetchProtocolExecute_SourceError_NoSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	source.EXPECT().FetchProtocol(gomock.Any(), "svc", gomock.Nil()).Return(nil, false, entities.ErrProtocolNotPublished)
	// store.Save не должен вызываться — рабочий контракт не затирается.

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "out.json"})

	assert.ErrorIs(t, err, entities.ErrProtocolNotPublished)
}

func TestFetchProtocolExecute_InvalidProtocol_NoSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	bad := &entities.Protocol{ServiceID: "svc", Document: []byte("<html>")}
	source.EXPECT().FetchProtocol(gomock.Any(), "svc", gomock.Nil()).Return(bad, false, nil)
	// невалидный контракт не сохраняется.

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "out.json"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestFetchProtocolExecute_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	protocol := &entities.Protocol{ServiceID: "svc", Document: []byte(validDoc)}
	storeErr := errors.New("disk full")
	source.EXPECT().FetchProtocol(gomock.Any(), "svc", gomock.Nil()).Return(protocol, false, nil)
	store.EXPECT().Save(gomock.Any(), protocol, "protocols").Return("", storeErr)

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols"})

	assert.ErrorIs(t, err, storeErr)
}
