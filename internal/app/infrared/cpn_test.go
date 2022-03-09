//go:generate mockgen -destination=cpn_mock_test.go -package=infrared_test . ConnProcessor,ProcessedConn
package infrared_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/haveachin/infrared/internal/app/infrared"
	"go.uber.org/zap"
)

func mockProcessConn(ctrl *gomock.Controller) *MockProcessedConn {
	pc := NewMockProcessedConn(ctrl)
	pc.EXPECT().ServerAddr().AnyTimes().Return("serverAddr")
	pc.EXPECT().Username().AnyTimes().Return("username")
	pc.EXPECT().GatewayID().AnyTimes().Return("gatewayId")
	pc.EXPECT().IsLoginRequest().AnyTimes().Return(false)
	return pc
}

func TestCPN_ListenAndServe(t *testing.T) {
	ctrl := gomock.NewController(t)
	tt := []struct {
		name    string
		err     error
		in      *MockConn
		out     *MockProcessedConn
		procDur time.Duration
		procErr error
	}{
		{
			name:    "ProcessConn",
			in:      mockConn(ctrl),
			out:     mockProcessConn(ctrl),
			procDur: time.Millisecond,
		},
		{
			name:    "ProcessConn_ConnTimesOut",
			in:      mockConn(ctrl),
			procDur: time.Millisecond * 2,
			procErr: errors.New(""),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cp := NewMockConnProcessor(ctrl)
			cp.EXPECT().ClientTimeout().Times(1).Return(time.Duration(0))
			cp.EXPECT().ProcessConn(tc.in).Times(1).Return(tc.out, tc.procErr)
			bus := NewMockBus(ctrl)
			bus.EXPECT().Push(infrared.PreConnProcessingEventTopic, gomock.Any()).
				Times(1).Return()

			if tc.err == nil {
				tc.in.EXPECT().SetDeadline(gomock.Any()).Times(1).Return(nil)
			}

			if tc.out == nil {
				tc.in.EXPECT().Close().Times(1).Return(nil)
			} else {
				tc.in.EXPECT().SetDeadline(time.Time{}).Times(1).Return(nil)
				bus.EXPECT().Push(infrared.PostConnProcessingEventTopic, gomock.Any()).
					Times(1).Return()
			}

			in := make(chan infrared.Conn)
			out := make(chan infrared.ProcessedConn)
			cpn := infrared.CPN{
				ConnProcessor: cp,
				In:            in,
				Out:           out,
				Logger:        zap.NewNop(),
				EventBus:      bus,
			}

			ctx, cancel := context.WithCancel(context.Background())
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				cpn.ListenAndServe(ctx)
				wg.Done()
			}()
			in <- tc.in
			cancel()

			if tc.out != nil {
				if <-out != tc.out {
					t.Fail()
				}
			}

			wg.Wait()
		})
	}
}
