package infrared

import (
	"errors"
	"sync/atomic"
)

type errorLocker struct {
	locker uint32
}

func (l *errorLocker) lock() error {
	isLocked := !atomic.CompareAndSwapUint32(&l.locker, 0, 1)
	if isLocked {
		return errors.New("is locked")
	}
	return nil
}

func (l *errorLocker) unlock() {
	atomic.StoreUint32(&l.locker, 0)
}
