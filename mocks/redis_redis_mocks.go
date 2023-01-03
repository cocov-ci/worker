// Code generated by MockGen. DO NOT EDIT.
// Source: redis/redis.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	redis "github.com/cocov-ci/worker/redis"
	gomock "github.com/golang/mock/gomock"
)

// RedisMock is a mock of Client interface.
type RedisMock struct {
	ctrl     *gomock.Controller
	recorder *RedisMockMockRecorder
}

// RedisMockMockRecorder is the mock recorder for RedisMock.
type RedisMockMockRecorder struct {
	mock *RedisMock
}

// NewRedisMock creates a new mock instance.
func NewRedisMock(ctrl *gomock.Controller) *RedisMock {
	mock := &RedisMock{ctrl: ctrl}
	mock.recorder = &RedisMockMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *RedisMock) EXPECT() *RedisMockMockRecorder {
	return m.recorder
}

// Next mocks base method.
func (m *RedisMock) Next() *redis.Job {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].(*redis.Job)
	return ret0
}

// Next indicates an expected call of Next.
func (mr *RedisMockMockRecorder) Next() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*RedisMock)(nil).Next))
}

// Ping mocks base method.
func (m *RedisMock) Ping() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Ping")
	ret0, _ := ret[0].(error)
	return ret0
}

// Ping indicates an expected call of Ping.
func (mr *RedisMockMockRecorder) Ping() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Ping", reflect.TypeOf((*RedisMock)(nil).Ping))
}

// Start mocks base method.
func (m *RedisMock) Start() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Start")
}

// Start indicates an expected call of Start.
func (mr *RedisMockMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*RedisMock)(nil).Start))
}

// Stop mocks base method.
func (m *RedisMock) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *RedisMockMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*RedisMock)(nil).Stop))
}

// Wait mocks base method.
func (m *RedisMock) Wait() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Wait")
}

// Wait indicates an expected call of Wait.
func (mr *RedisMockMockRecorder) Wait() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Wait", reflect.TypeOf((*RedisMock)(nil).Wait))
}
