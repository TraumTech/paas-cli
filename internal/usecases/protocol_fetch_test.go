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

const twoOpDoc = `{"openapi":"3.1.0","paths":{` +
	`"/a":{"get":{"operationId":"op-a","responses":{"200":{"description":"ok"}}}},` +
	`"/b":{"get":{"operationId":"op-b","responses":{"200":{"description":"ok"}}}}}}`

func TestFetchProtocolExecute_PartialSavesSelectedSubset(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	protocol := &entities.Protocol{ServiceID: "svc", ServiceName: "svc-name", Document: []byte(twoOpDoc)}
	source.EXPECT().FetchProtocol(gomock.Any(), "svc").Return(protocol, nil)
	store.EXPECT().Save(gomock.Any(), gomock.Any(), "protocols").
		DoAndReturn(func(_ context.Context, saved *entities.Protocol, _ string) (string, error) {
			assert.Contains(t, string(saved.Document), "op-a")
			assert.NotContains(t, string(saved.Document), "op-b")
			return "protocols/svc-name/openapi.json", nil
		})

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols", Methods: []string{"GET /a"}})
	require.NoError(t, err)
}

func TestFetchProtocolExecute_UnknownMethod_NoSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	protocol := &entities.Protocol{ServiceID: "svc", Document: []byte(twoOpDoc)}
	source.EXPECT().FetchProtocol(gomock.Any(), "svc").Return(protocol, nil)
	// store.Save не вызывается — несуществующий метод не даёт записать неполный срез.

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols", Methods: []string{"GET /x"}})

	var unknown *entities.UnknownMethodsError
	assert.ErrorAs(t, err, &unknown)
}

func TestFetchProtocolExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMockProtocolSource(ctrl)
	store := NewMockProtocolStore(ctrl)

	protocol := &entities.Protocol{ServiceID: "svc", ServiceName: "svc-name", VersionNumber: 3, Document: []byte(validDoc)}
	source.EXPECT().FetchProtocol(gomock.Any(), "svc").Return(protocol, nil)
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

	source.EXPECT().FetchProtocol(gomock.Any(), "svc").Return(nil, entities.ErrProtocolNotPublished)
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
	source.EXPECT().FetchProtocol(gomock.Any(), "svc").Return(bad, nil)
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
	source.EXPECT().FetchProtocol(gomock.Any(), "svc").Return(protocol, nil)
	store.EXPECT().Save(gomock.Any(), protocol, "protocols").Return("", storeErr)

	_, err := NewFetchProtocol(source, store).Execute(context.Background(),
		FetchProtocolInput{ServiceID: "svc", Destination: "protocols"})

	assert.ErrorIs(t, err, storeErr)
}
