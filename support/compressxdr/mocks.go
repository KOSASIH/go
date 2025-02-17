package compressxdr

import (
	"io"

	"github.com/stretchr/testify/mock"
)

type MockXDRDecoder struct {
	mock.Mock
}

func (m *MockXDRDecoder) ReadFrom(r io.Reader) (int64, error) {
	args := m.Called(r)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockXDRDecoder) Unzip(r io.Reader) ([]byte, error) {
	args := m.Called(r)
	return args.Get(0).([]byte), args.Error(1)
}
