// Code generated by MockGen. DO NOT EDIT.
// Source: docker/docker.go

// Package mocks is a generated GoMock package.
package mocks

import (
	bytes "bytes"
	reflect "reflect"

	docker "github.com/cocov-ci/worker/docker"
	gomock "github.com/golang/mock/gomock"
)

// DockerMock is a mock of Client interface.
type DockerMock struct {
	ctrl     *gomock.Controller
	recorder *DockerMockMockRecorder
}

// DockerMockMockRecorder is the mock recorder for DockerMock.
type DockerMockMockRecorder struct {
	mock *DockerMock
}

// NewDockerMock creates a new mock instance.
func NewDockerMock(ctrl *gomock.Controller) *DockerMock {
	mock := &DockerMock{ctrl: ctrl}
	mock.recorder = &DockerMockMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *DockerMock) EXPECT() *DockerMockMockRecorder {
	return m.recorder
}

// AbortAndRemove mocks base method.
func (m *DockerMock) AbortAndRemove(container *docker.CreateContainerResult) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AbortAndRemove", container)
	ret0, _ := ret[0].(error)
	return ret0
}

// AbortAndRemove indicates an expected call of AbortAndRemove.
func (mr *DockerMockMockRecorder) AbortAndRemove(container interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AbortAndRemove", reflect.TypeOf((*DockerMock)(nil).AbortAndRemove), container)
}

// ContainerStart mocks base method.
func (m *DockerMock) ContainerStart(id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerStart", id)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerStart indicates an expected call of ContainerStart.
func (mr *DockerMockMockRecorder) ContainerStart(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerStart", reflect.TypeOf((*DockerMock)(nil).ContainerStart), id)
}

// ContainerWait mocks base method.
func (m *DockerMock) ContainerWait(id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContainerWait", id)
	ret0, _ := ret[0].(error)
	return ret0
}

// ContainerWait indicates an expected call of ContainerWait.
func (mr *DockerMockMockRecorder) ContainerWait(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContainerWait", reflect.TypeOf((*DockerMock)(nil).ContainerWait), id)
}

// CreateContainer mocks base method.
func (m *DockerMock) CreateContainer(info *docker.RunInformation) (*docker.CreateContainerResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateContainer", info)
	ret0, _ := ret[0].(*docker.CreateContainerResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateContainer indicates an expected call of CreateContainer.
func (mr *DockerMockMockRecorder) CreateContainer(info interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateContainer", reflect.TypeOf((*DockerMock)(nil).CreateContainer), info)
}

// GetContainerOutput mocks base method.
func (m *DockerMock) GetContainerOutput(result *docker.CreateContainerResult) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetContainerOutput", result)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetContainerOutput indicates an expected call of GetContainerOutput.
func (mr *DockerMockMockRecorder) GetContainerOutput(result interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetContainerOutput", reflect.TypeOf((*DockerMock)(nil).GetContainerOutput), result)
}

// GetContainerResult mocks base method.
func (m *DockerMock) GetContainerResult(id string) (*bytes.Buffer, int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetContainerResult", id)
	ret0, _ := ret[0].(*bytes.Buffer)
	ret1, _ := ret[1].(int)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetContainerResult indicates an expected call of GetContainerResult.
func (mr *DockerMockMockRecorder) GetContainerResult(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetContainerResult", reflect.TypeOf((*DockerMock)(nil).GetContainerResult), id)
}

// PrepareVolume mocks base method.
func (m *DockerMock) PrepareVolume(brotliPath string) (*docker.PrepareVolumeResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrepareVolume", brotliPath)
	ret0, _ := ret[0].(*docker.PrepareVolumeResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PrepareVolume indicates an expected call of PrepareVolume.
func (mr *DockerMockMockRecorder) PrepareVolume(brotliPath interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrepareVolume", reflect.TypeOf((*DockerMock)(nil).PrepareVolume), brotliPath)
}

// PullImage mocks base method.
func (m *DockerMock) PullImage(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PullImage", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// PullImage indicates an expected call of PullImage.
func (mr *DockerMockMockRecorder) PullImage(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PullImage", reflect.TypeOf((*DockerMock)(nil).PullImage), name)
}

// RemoveVolume mocks base method.
func (m *DockerMock) RemoveVolume(vol *docker.PrepareVolumeResult) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveVolume", vol)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveVolume indicates an expected call of RemoveVolume.
func (mr *DockerMockMockRecorder) RemoveVolume(vol interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveVolume", reflect.TypeOf((*DockerMock)(nil).RemoveVolume), vol)
}

// TerminateContainer mocks base method.
func (m *DockerMock) TerminateContainer(v *docker.CreateContainerResult) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "TerminateContainer", v)
}

// TerminateContainer indicates an expected call of TerminateContainer.
func (mr *DockerMockMockRecorder) TerminateContainer(v interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TerminateContainer", reflect.TypeOf((*DockerMock)(nil).TerminateContainer), v)
}
