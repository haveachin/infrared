//go:generate mockgen -destination=net_mock_test.go -package=infrared_test net Conn,Addr
//go:generate mockgen -destination=event_mock_test.go -package=infrared_test github.com/haveachin/infrared/pkg/event Bus
package infrared_test

import (
	gomock "github.com/golang/mock/gomock"
)

func mockAddr(ctrl *gomock.Controller) *MockAddr {
	addr := NewMockAddr(ctrl)
	addr.EXPECT().String().AnyTimes().Return("ip:port")
	addr.EXPECT().Network().AnyTimes().Return("network")
	return addr
}

func mockConn(ctrl *gomock.Controller) *MockConn {
	c := NewMockConn(ctrl)
	c.EXPECT().LocalAddr().AnyTimes().Return(mockAddr(ctrl))
	c.EXPECT().RemoteAddr().AnyTimes().Return(mockAddr(ctrl))
	return c
}
