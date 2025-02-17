// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockS3 struct {
	mock.Mock
	s3iface.S3API
}

type MockS3HttpProxy struct {
	mock.Mock
	s3HttpProxy
}

func (m *MockS3HttpProxy) Send(input *s3.GetObjectInput) (io.ReadCloser, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func TestWriteACLRuleOverride(t *testing.T) {

	mockS3 := &MockS3{}
	s3Storage := S3Storage{
		ctx:              context.Background(),
		svc:              mockS3,
		bucket:           "bucket",
		prefix:           "prefix",
		unsignedRequests: false,
		writeACLrule:     s3.ObjectCannedACLBucketOwnerFullControl,
	}

	aclRule := s3Storage.GetACLWriteRule()
	assert.Equal(t, aclRule, s3.ObjectCannedACLBucketOwnerFullControl)
}

func TestWriteACLRuleDefault(t *testing.T) {

	mockS3 := &MockS3{}
	s3Storage := S3Storage{
		ctx:              context.Background(),
		svc:              mockS3,
		bucket:           "bucket",
		prefix:           "prefix",
		unsignedRequests: false,
		writeACLrule:     "",
	}

	aclRule := s3Storage.GetACLWriteRule()
	assert.Equal(t, aclRule, s3.ObjectCannedACLPublicRead)
}

func TestGetFileNotFound(t *testing.T) {
	mockS3 := &MockS3{}
	mockS3HttpProxy := &MockS3HttpProxy{}

	mockS3HttpProxy.On("Send", mock.Anything).Return(nil,
		awserr.New(s3.ErrCodeNoSuchKey, "message", errors.New("not found")))

	s3Storage := S3Storage{
		ctx:              context.Background(),
		svc:              mockS3,
		bucket:           "bucket",
		prefix:           "prefix",
		unsignedRequests: false,
		writeACLrule:     "",
		s3Http:           mockS3HttpProxy,
	}

	_, err := s3Storage.GetFile("path")

	assert.Equal(t, err, os.ErrNotExist)
}

func TestGetFileFound(t *testing.T) {
	mockS3 := &MockS3{}
	mockS3HttpProxy := &MockS3HttpProxy{}
	testCloser := io.NopCloser(strings.NewReader(""))

	mockS3HttpProxy.On("Send", mock.Anything).Return(testCloser, nil)

	s3Storage := S3Storage{
		ctx:              context.Background(),
		svc:              mockS3,
		bucket:           "bucket",
		prefix:           "prefix",
		unsignedRequests: false,
		writeACLrule:     "",
		s3Http:           mockS3HttpProxy,
	}

	closer, err := s3Storage.GetFile("path")
	assert.Nil(t, err)
	assert.Equal(t, closer, testCloser)
}
