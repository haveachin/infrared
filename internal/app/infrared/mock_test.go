//go:generate mockgen -destination=infrared_mock_test.go -package=infrared_test github.com/haveachin/infrared/internal/app/infrared Conn,ConnProcessor,Player,Server,Version
//go:generate mockgen -destination=net_mock_test.go -package=infrared_test net Addr
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

func mockVersion(ctrl *gomock.Controller) *MockVersion {
	v := NewMockVersion(ctrl)
	v.EXPECT().Name().AnyTimes().Return("version")
	v.EXPECT().ProtocolNumber().AnyTimes().Return(int32(0))
	return v
}

func mockPlayer(ctrl *gomock.Controller) *MockPlayer {
	p := NewMockPlayer(ctrl)
	p.EXPECT().LocalAddr().AnyTimes().Return(mockAddr(ctrl))
	p.EXPECT().RemoteAddr().AnyTimes().Return(mockAddr(ctrl))
	p.EXPECT().Edition().AnyTimes().Return(infrared.JavaEdition)
	p.EXPECT().RequestedAddr().AnyTimes().Return("requestedAddr")
	p.EXPECT().MatchedAddr().AnyTimes().Return("matchedAddr")
	p.EXPECT().Username().AnyTimes().Return("username")
	p.EXPECT().GatewayID().AnyTimes().Return("gatewayId")
	p.EXPECT().IsLoginRequest().AnyTimes().Return(false)
	p.EXPECT().Version().AnyTimes().Return(mockVersion(ctrl))
	return p
}
