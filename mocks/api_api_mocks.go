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

// GetSecret mocks base method.
func (m *APIMock) GetSecret(secret *redis.Mount) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSecret", secret)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSecret indicates an expected call of GetSecret.
func (mr *APIMockMockRecorder) GetSecret(secret interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSecret", reflect.TypeOf((*APIMock)(nil).GetSecret), secret)
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
func (m *APIMock) PushIssues(job *redis.Job, plugin string, issues any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PushIssues", job, plugin, issues)
	ret0, _ := ret[0].(error)
	return ret0
}

// PushIssues indicates an expected call of PushIssues.
func (mr *APIMockMockRecorder) PushIssues(job, plugin, issues interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PushIssues", reflect.TypeOf((*APIMock)(nil).PushIssues), job, plugin, issues)
}

// SetCheckCanceled mocks base method.
func (m *APIMock) SetCheckCanceled(job *redis.Job, plugin string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetCheckCanceled", job, plugin)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCheckCanceled indicates an expected call of SetCheckCanceled.
func (mr *APIMockMockRecorder) SetCheckCanceled(job, plugin interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCheckCanceled", reflect.TypeOf((*APIMock)(nil).SetCheckCanceled), job, plugin)
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

// SetSetRunning mocks base method.
func (m *APIMock) SetSetRunning(job *redis.Job) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetSetRunning", job)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetSetRunning indicates an expected call of SetSetRunning.
func (mr *APIMockMockRecorder) SetSetRunning(job interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSetRunning", reflect.TypeOf((*APIMock)(nil).SetSetRunning), job)
}

// WrapUp mocks base method.
func (m *APIMock) WrapUp(job *redis.Job) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WrapUp", job)
	ret0, _ := ret[0].(error)
	return ret0
}

// WrapUp indicates an expected call of WrapUp.
func (mr *APIMockMockRecorder) WrapUp(job interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WrapUp", reflect.TypeOf((*APIMock)(nil).WrapUp), job)
}
