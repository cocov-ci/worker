// Code generated by MockGen. DO NOT EDIT.
// Source: api/api.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	redis "github.com/cocov-ci/worker/redis"
	gomock "github.com/golang/mock/gomock"
)

// APIMock is a mock of Client interface.
type APIMock struct {
	ctrl     *gomock.Controller
	recorder *APIMockMockRecorder
}

// APIMockMockRecorder is the mock recorder for APIMock.
type APIMockMockRecorder struct {
	mock *APIMock
}

// NewAPIMock creates a new mock instance.
func NewAPIMock(ctrl *gomock.Controller) *APIMock {
	mock := &APIMock{ctrl: ctrl}
	mock.recorder = &APIMockMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *APIMock) EXPECT() *APIMockMockRecorder {
	return m.recorder
}

// Ping mocks base method.
func (m *APIMock) Ping() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping")
	ret0, _ := ret[0].(error)
	return ret0
}

// Ping indicates an expected call of Ping.
func (mr *APIMockMockRecorder) Ping() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*APIMock)(nil).Ping))
}

// PushIssues mocks base method.
func (m *APIMock) PushIssues(job *redis.Job, issues map[string]interface{}, status string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PushIssues", job, issues, status)
	ret0, _ := ret[0].(error)
	return ret0
}

// PushIssues indicates an expected call of PushIssues.
func (mr *APIMockMockRecorder) PushIssues(job, issues, status interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PushIssues", reflect.TypeOf((*APIMock)(nil).PushIssues), job, issues, status)
}

// SetCheckError mocks base method.
func (m *APIMock) SetCheckError(job *redis.Job, plugin, reason string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetCheckError", job, plugin, reason)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCheckError indicates an expected call of SetCheckError.
func (mr *APIMockMockRecorder) SetCheckError(job, plugin, reason interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCheckError", reflect.TypeOf((*APIMock)(nil).SetCheckError), job, plugin, reason)
}

// SetCheckRunning mocks base method.
func (m *APIMock) SetCheckRunning(job *redis.Job, plugin string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetCheckRunning", job, plugin)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCheckRunning indicates an expected call of SetCheckRunning.
func (mr *APIMockMockRecorder) SetCheckRunning(job, plugin interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCheckRunning", reflect.TypeOf((*APIMock)(nil).SetCheckRunning), job, plugin)
}

// SetCheckSucceeded mocks base method.
func (m *APIMock) SetCheckSucceeded(job *redis.Job, plugin string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetCheckSucceeded", job, plugin)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCheckSucceeded indicates an expected call of SetCheckSucceeded.
func (mr *APIMockMockRecorder) SetCheckSucceeded(job, plugin interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCheckSucceeded", reflect.TypeOf((*APIMock)(nil).SetCheckSucceeded), job, plugin)
}
