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
	if s.InitMs < 80 && s.AvgExec < 80 {
		return 500 * time.Millisecond
	}
	return 1500 * time.Millisecond
}

func (s *BaseScheduler) GetIdleDurationBeforeGC(inst *model.Instance, remain int) time.Duration {
	if s.d.fastTerm() {
		return 10 * time.Millisecond
	}
	if inst.KeepAliveMs > 0 && inst.LastStart.Sub(inst.LastIdleTime).Milliseconds() == 0 {
		return time.Duration(inst.KeepAliveMs) * time.Millisecond
	}
	if s.Rule.Valid && s.Rule.GcMill > 0 {
		if remain == 1 && s.Rule.GcSingleMill > 0 {
			return time.Duration(s.Rule.GcSingleMill) * time.Millisecond
		}
		return time.Duration(s.Rule.GcMill) * time.Millisecond
	}
	return s.config.IdleDurationBeforeGC
}
