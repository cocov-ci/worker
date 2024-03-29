// Code generated by MockGen. DO NOT EDIT.
// Source: storage/base.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// StorageMock is a mock of Base interface.
type StorageMock struct {
	ctrl     *gomock.Controller
	recorder *StorageMockMockRecorder
}

// StorageMockMockRecorder is the mock recorder for StorageMock.
type StorageMockMockRecorder struct {
	mock *StorageMock
}

// NewStorageMock creates a new mock instance.
func NewStorageMock(ctrl *gomock.Controller) *StorageMock {
	mock := &StorageMock{ctrl: ctrl}
	mock.recorder = &StorageMockMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *StorageMock) EXPECT() *StorageMockMockRecorder {
	return m.recorder
}

// CommitPath mocks base method.
func (m *StorageMock) CommitPath(repository, commitish string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitPath", repository, commitish)
	ret0, _ := ret[0].(string)
	return ret0
}

// CommitPath indicates an expected call of CommitPath.
func (mr *StorageMockMockRecorder) CommitPath(repository, commitish interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitPath", reflect.TypeOf((*StorageMock)(nil).CommitPath), repository, commitish)
}

// DownloadCommit mocks base method.
func (m *StorageMock) DownloadCommit(repository, commitish, into string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DownloadCommit", repository, commitish, into)
	ret0, _ := ret[0].(error)
	return ret0
}

// DownloadCommit indicates an expected call of DownloadCommit.
func (mr *StorageMockMockRecorder) DownloadCommit(repository, commitish, into interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DownloadCommit", reflect.TypeOf((*StorageMock)(nil).DownloadCommit), repository, commitish, into)
}

// RepositoryPath mocks base method.
func (m *StorageMock) RepositoryPath(repository string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RepositoryPath", repository)
	ret0, _ := ret[0].(string)
	return ret0
}

// RepositoryPath indicates an expected call of RepositoryPath.
func (mr *StorageMockMockRecorder) RepositoryPath(repository interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RepositoryPath", reflect.TypeOf((*StorageMock)(nil).RepositoryPath), repository)
}
