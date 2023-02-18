package infrared_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/haveachin/infrared/internal/app/infrared"
	"go.uber.org/zap"
)

func TestCPN_ListenAndServe(t *testing.T) {
	ctrl := gomock.NewController(t)
	tt := []struct {
		name    string
		err     error
		in      *MockConn
		out     *MockPlayer
		procDur time.Duration
		procErr error
	}{
		{
			name:    "ProcessConn",
			in:      mockConn(ctrl),
			out:     mockPlayer(ctrl),
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

			if tc.err == nil {
				tc.in.EXPECT().SetDeadline(gomock.Any()).Times(1).Return(nil)
			}

			if tc.out == nil {
				tc.in.EXPECT().Close().Times(1).Return(nil)
			} else {
				tc.in.EXPECT().SetDeadline(time.Time{}).Times(1).Return(nil)
				//infrared.PostProcessingEvent.EXPECT().Push(gomock.Any(), infrared.PostProcessingPayload).Times(1)
			}

			in := make(chan infrared.Conn)
			out := make(chan infrared.Player, 1)
			cpn := infrared.CPN{
				ConnProcessor: cp,
				In:            in,
				Out:           out,
				Logger:        zap.NewNop(),
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			quit := make(chan bool)
			go func() {
				cpn.ListenAndServe(quit)
				wg.Done()
			}()
			in <- tc.in
			quit <- true

			if tc.out != nil {
				if <-out != tc.out {
					t.Fail()
				}
			}

			wg.Wait()
		})
	}
}
