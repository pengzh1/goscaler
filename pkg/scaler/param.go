package scaler

import (
	"time"
)

func (s *BaseScheduler) GetGcInterval() time.Duration {
	//if avgExec > 1000 && avgExec/initDuration >= 1 {
	//	return 2 * time.Second
	//}
	//
	//if avgExec > 500 && avgExec/initDuration >= 1 {
	//	return 5 * time.Second
	//}
	return 2 * time.Second
}

func (s *BaseScheduler) GetIdleDurationBeforeGC() time.Duration {
	//if avgExec > 1000 && avgExec/initDuration >= 1 {
	//	return 2 * time.Second
	//}
	//if avgExec > 500 && avgExec/initDuration >= 1 {
	//	return 5 * time.Second
	//}
	//curSec := time.Now().Sub(model.CreateTime).Milliseconds() / 1000
	//start := curSec - 40
	//if start < 0 {
	//	start = 0
	//}
	//cnt := int64(0)
	//for i := start; i < curSec; i++ {
	//	if useTime[start] > 0 {
	//		cnt += 1
	//	}
	//}
	//if cnt > 1 && curSec-start > 5 {
	//	return time.Duration(2*(curSec-start)/cnt) * time.Second
	//}
	return s.config.IdleDurationBeforeGC
}
