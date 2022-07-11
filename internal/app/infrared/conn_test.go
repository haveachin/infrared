package infrared_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/haveachin/infrared/internal/app/infrared"
)

func TestExecuteMessageTemplate(t *testing.T) {
	tt := []struct {
		name string
		tmpl string
		msg  string
	}{
		{
			name: "Username",
			tmpl: "{{username}}",
			msg:  "username",
		},
		{
			name: "RemoteAddress",
			tmpl: "{{remoteAddress}}",
			msg:  "ip:port",
		},
		{
			name: "LocalAddress",
			tmpl: "{{localAddress}}",
			msg:  "ip:port",
		},
		{
			name: "ServerDomain",
			tmpl: "{{serverDomain}}",
			msg:  "serverAddr",
		},
		{
			name: "GatewayID",
			tmpl: "{{gatewayId}}",
			msg:  "gatewayId",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			pc := mockProcessedConn(ctrl)

			msg := infrared.ExecuteMessageTemplate(tc.tmpl, pc)
			if tc.msg != msg {
				t.Fail()
			}
		})
	}
}
