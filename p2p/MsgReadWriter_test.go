package p2p

import mock "github.com/stretchr/testify/mock"

import types "github.com/aergoio/aergo/types"

// MockMsgReadWriter is an autogenerated mock type for the MsgReadWriter type
type MockMsgReadWriter struct {
	mock.Mock
}

// ReadMsg provides a mock function with given fields:
func (_m *MockMsgReadWriter) ReadMsg() (*types.P2PMessage, error) {
	ret := _m.Called()

	var r0 *types.P2PMessage
	if rf, ok := ret.Get(0).(func() *types.P2PMessage); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.P2PMessage)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WriteMsg provides a mock function with given fields: header
func (_m *MockMsgReadWriter) WriteMsg(header *types.P2PMessage) error {
	ret := _m.Called(header)

	var r0 error
	if rf, ok := ret.Get(0).(func(*types.P2PMessage) error); ok {
		r0 = rf(header)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}