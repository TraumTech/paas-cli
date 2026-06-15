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

const validContract = `{"openapi":"3.1.0","paths":{"/x":{}}}`

func TestRegisterDependencyExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte(validContract), nil)
	dependency := &entities.Dependency{ConsumerVersionID: "ver-1", ProducerServiceID: "prod"}
	registrar.EXPECT().
		RegisterDependency(gomock.Any(), "svc", "ver-1", "prod", []byte(validContract)).
		Return(dependency, nil)

	got, err := NewRegisterDependency(reader, registrar).Execute(context.Background(),
		RegisterDependencyInput{ServiceID: "svc", VersionID: "ver-1", ProducerServiceID: "prod", ContractPath: "contract.json"})

	require.NoError(t, err)
	assert.Same(t, dependency, got)
}

func TestRegisterDependencyExecute_ReadError_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)
	// RegisterDependency не вызывается — файл не прочитан.

	readErr := errors.New("no such file")
	reader.EXPECT().Read(gomock.Any(), "missing.json").Return(nil, readErr)

	_, err := NewRegisterDependency(reader, registrar).Execute(context.Background(),
		RegisterDependencyInput{ServiceID: "svc", VersionID: "ver-1", ProducerServiceID: "prod", ContractPath: "missing.json"})

	assert.ErrorIs(t, err, readErr)
}

func TestRegisterDependencyExecute_InvalidSnapshot_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)
	// RegisterDependency не вызывается — снимок не похож на контракт.

	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte(`{"not":"openapi"}`), nil)

	_, err := NewRegisterDependency(reader, registrar).Execute(context.Background(),
		RegisterDependencyInput{ServiceID: "svc", VersionID: "ver-1", ProducerServiceID: "prod", ContractPath: "contract.json"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestRegisterDependencyExecute_EmptySnapshot_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte("   "), nil)

	_, err := NewRegisterDependency(reader, registrar).Execute(context.Background(),
		RegisterDependencyInput{ServiceID: "svc", VersionID: "ver-1", ProducerServiceID: "prod", ContractPath: "contract.json"})

	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
}

func TestRegisterDependencyExecute_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte(validContract), nil)
	srcErr := errors.New("boom")
	registrar.EXPECT().RegisterDependency(gomock.Any(), "svc", "ver-1", "prod", gomock.Any()).Return(nil, srcErr)

	_, err := NewRegisterDependency(reader, registrar).Execute(context.Background(),
		RegisterDependencyInput{ServiceID: "svc", VersionID: "ver-1", ProducerServiceID: "prod", ContractPath: "contract.json"})

	assert.ErrorIs(t, err, srcErr)
}
