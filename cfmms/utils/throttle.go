package utils

import (
	"sync"
	"time"
)

type Throttle struct {
	Enabled               bool
	LastRequestTimestamp  time.Time
	RequestPerSecondLimit uint64
	RequestPerSecond      uint64
	mu                    sync.Mutex
}

func NewThrottle(requests_per_second_limit uint64) *Throttle {

	if requests_per_second_limit > 0 {
		return &Throttle{
			Enabled:               true,
			LastRequestTimestamp:  time.Now(),
			RequestPerSecondLimit: requests_per_second_limit,
			RequestPerSecond:      0,
		}
	} else {
		return &Throttle{
			Enabled:               false,
			LastRequestTimestamp:  time.Now(),
			RequestPerSecondLimit: requests_per_second_limit,
			RequestPerSecond:      0,
		}
	}

}

func (receiver *Throttle) IncrementOrSleep(inc uint64) {
	receiver.mu.Lock()
	defer receiver.mu.Unlock()
	time_elapsed := time.Since(receiver.LastRequestTimestamp).Milliseconds()
	if receiver.Enabled && time_elapsed < 1000 {
		if receiver.RequestPerSecond >= receiver.RequestPerSecondLimit {
			time.Sleep(time.Second)
			receiver.RequestPerSecond = 0
			receiver.LastRequestTimestamp = time.Now()
		} else {
			receiver.RequestPerSecond += inc
		}
	}
}
