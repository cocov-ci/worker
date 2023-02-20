// Code generated by MockGen. DO NOT EDIT.
// Source: runner/scheduler.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	redis "github.com/cocov-ci/worker/redis"
	gomock "github.com/golang/mock/gomock"
)

// IndirectScheduler is a mock of IndirectScheduler interface.
type IndirectScheduler struct {
	ctrl     *gomock.Controller
	recorder *IndirectSchedulerMockRecorder
}

// IndirectSchedulerMockRecorder is the mock recorder for IndirectScheduler.
type IndirectSchedulerMockRecorder struct {
	mock *IndirectScheduler
}

// NewIndirectScheduler creates a new mock instance.
func NewIndirectScheduler(ctrl *gomock.Controller) *IndirectScheduler {
	mock := &IndirectScheduler{ctrl: ctrl}
	mock.recorder = &IndirectSchedulerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *IndirectScheduler) EXPECT() *IndirectSchedulerMockRecorder {
	return m.recorder
}

// DeregisterJob mocks base method.
func (m *IndirectScheduler) DeregisterJob(j *redis.Job, workerID int) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "DeregisterJob", j, workerID)
}

// DeregisterJob indicates an expected call of DeregisterJob.
func (mr *IndirectSchedulerMockRecorder) DeregisterJob(j, workerID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeregisterJob", reflect.TypeOf((*IndirectScheduler)(nil).DeregisterJob), j, workerID)
}

// RegisterJob mocks base method.
func (m *IndirectScheduler) RegisterJob(j *redis.Job, workerID int) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterJob", j, workerID)
}

// RegisterJob indicates an expected call of RegisterJob.
func (mr *IndirectSchedulerMockRecorder) RegisterJob(j, workerID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterJob", reflect.TypeOf((*IndirectScheduler)(nil).RegisterJob), j, workerID)
}

// Mockrunnable is a mock of runnable interface.
type Mockrunnable struct {
	ctrl     *gomock.Controller
	recorder *MockrunnableMockRecorder
}

// MockrunnableMockRecorder is the mock recorder for Mockrunnable.
type MockrunnableMockRecorder struct {
	mock *Mockrunnable
}

// NewMockrunnable creates a new mock instance.
func NewMockrunnable(ctrl *gomock.Controller) *Mockrunnable {
	mock := &Mockrunnable{ctrl: ctrl}
	mock.recorder = &MockrunnableMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockrunnable) EXPECT() *MockrunnableMockRecorder {
	return m.recorder
}

// CancelJob mocks base method.
func (m *Mockrunnable) CancelJob(jobID string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CancelJob", jobID)
}

// CancelJob indicates an expected call of CancelJob.
func (mr *MockrunnableMockRecorder) CancelJob(jobID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelJob", reflect.TypeOf((*Mockrunnable)(nil).CancelJob), jobID)
}

// Run mocks base method.
func (m *Mockrunnable) Run() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Run")
}

// Run indicates an expected call of Run.
func (mr *MockrunnableMockRecorder) Run() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*Mockrunnable)(nil).Run))
}
