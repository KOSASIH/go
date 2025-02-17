package ledgerbackend

import (
	"context"
	"io/fs"
	"os"

	"github.com/stretchr/testify/mock"
)

type isDirImpl bool

func (i isDirImpl) IsDir() bool {
	return bool(i)
}

type mockSystemCaller struct {
	mock.Mock
}

func (m *mockSystemCaller) removeAll(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *mockSystemCaller) writeFile(filename string, data []byte, perm fs.FileMode) error {
	args := m.Called(filename, data, perm)
	return args.Error(0)
}

func (m *mockSystemCaller) mkdirAll(path string, perm os.FileMode) error {
	args := m.Called(path, perm)
	return args.Error(0)
}

func (m *mockSystemCaller) stat(name string) (isDir, error) {
	args := m.Called(name)
	return args.Get(0).(isDir), args.Error(1)
}

func (m *mockSystemCaller) command(ctx context.Context, name string, arg ...string) cmdI {
	a := []interface{}{ctx, name}
	for _, ar := range arg {
		a = append(a, ar)
	}
	args := m.Called(a...)
	return args.Get(0).(cmdI)
}
