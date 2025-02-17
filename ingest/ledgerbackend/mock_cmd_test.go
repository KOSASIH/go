package ledgerbackend

import (
	"os"

	"github.com/stretchr/testify/mock"
)

type mockCmd struct {
	mock.Mock
}

func (m *mockCmd) Output() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockCmd) Wait() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCmd) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCmd) Run() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCmd) setDir(dir string) {
	m.Called(dir)
}

func (m *mockCmd) setLogLineWriter(logWriter *logLineWriter) {
	m.Called(logWriter)
}

func (m *mockCmd) setExtraFiles(files []*os.File) {
	m.Called(files)
}

func simpleCommandMock() *mockCmd {
	cmdMock := &mockCmd{}
	cmdMock.On("setDir", mock.Anything)
	cmdMock.On("setLogLineWriter", mock.Anything)
	cmdMock.On("setExtraFiles", mock.Anything)
	cmdMock.On("Start").Return(nil)
	return cmdMock
}
