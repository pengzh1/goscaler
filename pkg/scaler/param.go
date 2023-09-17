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
	if s.Rule.Valid {
		return time.Duration(s.Rule.GcSec) * time.Second
	}
	return s.config.IdleDurationBeforeGC
}
