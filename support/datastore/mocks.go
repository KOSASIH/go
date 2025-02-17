package datastore

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
)

// MockDataStore is a mock implementation for the Storage interface.
type MockDataStore struct {
	mock.Mock
}

func (m *MockDataStore) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockDataStore) Size(ctx context.Context, path string) (int64, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDataStore) GetFileMetadata(ctx context.Context, path string) (map[string]string, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockDataStore) GetFile(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	closer := (io.ReadCloser)(nil)
	if args.Get(0) != nil {
		closer = args.Get(0).(io.ReadCloser)
	}
	return closer, args.Error(1)
}

func (m *MockDataStore) PutFile(ctx context.Context, path string, in io.WriterTo, metadata map[string]string) error {
	args := m.Called(ctx, path, in, metadata)
	return args.Error(0)
}

func (m *MockDataStore) PutFileIfNotExists(ctx context.Context, path string, in io.WriterTo, metadata map[string]string) (bool, error) {
	args := m.Called(ctx, path, in, metadata)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockDataStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDataStore) GetSchema() DataStoreSchema {
	args := m.Called()
	return args.Get(0).(DataStoreSchema)
}

type MockResumableManager struct {
	mock.Mock
}

func (m *MockResumableManager) FindStart(ctx context.Context, start, end uint32) (absentLedger uint32, ok bool, err error) {
	a := m.Called(ctx, start, end)
	return a.Get(0).(uint32), a.Get(1).(bool), a.Error(2)
}

// ensure that the MockClient implements ClientInterface
var _ ResumableManager = &MockResumableManager{}
var _ DataStore = &MockDataStore{}
