package scaler

import (
	"github.com/AliyunContainerService/scaler/go/pkg/model"
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

func (s *BaseScheduler) GetIdleDurationBeforeGC(inst *model.Instance) time.Duration {
	if inst.KeepAliveMs > 0 && inst.LastStart.Sub(inst.LastIdleTime).Milliseconds() == 0 {
		return time.Duration(inst.KeepAliveMs) * time.Millisecond
	}
	if s.Rule.Valid && s.Rule.GcSec > 0 {
		return time.Duration(s.Rule.GcSec) * time.Second
	}
	return s.config.IdleDurationBeforeGC
}
