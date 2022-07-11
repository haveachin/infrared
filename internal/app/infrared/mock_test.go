//go:generate mockgen -destination=infrared_mock_test.go -package=infrared_test github.com/haveachin/infrared/internal/app/infrared Conn,ConnProcessor,ProcessedConn,Server
//go:generate mockgen -destination=net_mock_test.go -package=infrared_test net Addr
//go:generate mockgen -destination=event_mock_test.go -package=infrared_test github.com/haveachin/infrared/pkg/event Bus
package infrared_test

import (
	gomock "github.com/golang/mock/gomock"
	"github.com/haveachin/infrared/internal/app/infrared"
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
	c.EXPECT().Edition().AnyTimes().Return(infrared.JavaEdition)
	return c
}

func mockProcessedConn(ctrl *gomock.Controller) *MockProcessedConn {
	pc := NewMockProcessedConn(ctrl)
	pc.EXPECT().LocalAddr().AnyTimes().Return(mockAddr(ctrl))
	pc.EXPECT().RemoteAddr().AnyTimes().Return(mockAddr(ctrl))
	pc.EXPECT().Edition().AnyTimes().Return(infrared.JavaEdition)
	pc.EXPECT().ServerAddr().AnyTimes().Return("serverAddr")
	pc.EXPECT().Username().AnyTimes().Return("username")
	pc.EXPECT().GatewayID().AnyTimes().Return("gatewayId")
	pc.EXPECT().IsLoginRequest().AnyTimes().Return(false)
	return pc
}
