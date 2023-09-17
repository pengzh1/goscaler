package scaler

import (
	"context"
	"github.com/AliyunContainerService/scaler/go/pkg/kmeans"
	m2 "github.com/AliyunContainerService/scaler/go/pkg/model"
	"github.com/google/uuid"
	"sort"
	"time"
)

func (d *Dispatcher) runGcChecker(ctx context.Context) {
	tick := time.NewTicker(120 * time.Millisecond)
	for {
		select {
		case <-tick.C:
			ss := make([]*BaseScheduler, 0, d.ScalerSize.Load())
			d.ScalerMap.Range(func(key, value any) bool {
				if value != nil {
					ss = append(ss, value.(*BaseScheduler))
				}
				return true
			})
			for _, s := range ss {
				d.runGc(s)
			}
		case <-ctx.Done():
			return
		}
	}
}
func (d *Dispatcher) runGc(s *BaseScheduler) {
	start := time.Now()
	if s.OnCreate.Load() < s.OnWait.Load() && s.OnCreate.Load() < 100 {
		// 1.onCreate小于onWait，触发一次创建检查事件
		createEvent := &CreateEvent{
			InstanceId: "sup" + uuid.NewString(),
			RequestId:  uuid.NewString(),
			Scaler:     s,
		}
		d.WaitCheckChan <- createEvent
		m2.Printf("supplementCreate" + s.MetaData.Key)
		m2.Printf("gcRunEnd%s,%dus", s.MetaData.Key, time.Since(start).Microseconds())
		s.GcTime += uint64(time.Since(start).Microseconds())
		s.GcCnt += 1
		return
	} else if s.OnWait.Load() == 0 {
		// 2.onWait=0，检查idleChan是否已清空
	loop:
		for {
			select {
			case ret := <-s.IdleChan:
				s.IdleSize.Add(-1)
				m2.ReqLog("", s.MetaData.Key, "", "fastConsume11", time.Since(ret.Instance.LastIdleTime).Milliseconds())
				d.transferDelete("sup"+uuid.NewString(), ret.Instance.Slot.Id, ret.Instance.Id, s.MetaData.Key, "sup")
				m2.Printf("supplementDelete %s,%s,%s", s.MetaData.Key, ret.Instance.Id, ret.Instance.Slot.Id)
			default:
				break loop
			}
		}
	}
	// 3.gc
	d.GC(s)
	// 4.onWait<onCreate，通知较新的create协程尽快销毁实例
	if s.OnWait.Load() < s.OnCreate.Load() {
		s.CheckLock.Lock()
		if s.OnWait.Load() < s.OnCreate.Load() {
			var arr []int64
			s.CheckChan.Range(func(key, value any) bool {
				arr = append(arr, key.(int64))
				return true
			})
			sort.Slice(arr, func(i, j int) bool {
				return arr[i] < arr[j]
			})
			size := s.OnWait.Load()
			if size < int32(len(arr)) {
				dropIdx := arr[size]
				s.CheckChan.Range(func(key, value any) bool {
					m2.Printf("sendCheck")
					if key.(int64) >= dropIdx {
						select {
						case value.(chan struct{}) <- struct{}{}:
							m2.Printf("sendCheckSuc")
						default:
							m2.Printf("sendCheckDrop")
						}
					}
					return true
				})
			}
		}
		s.CheckLock.Unlock()
	}
	s.GcTime += uint64(time.Since(start).Microseconds())
	s.GcCnt += 1
}

func (d *Dispatcher) GC(s *BaseScheduler) {
	if time.Since(s.LastKm) > 10*time.Second {
		s.LastKm = time.Now()
		// 准入条件，请求记录大于200，或创建时间大于5分钟且请求数大于1
		if s.ReqCnt >= 10 || (time.Since(s.CreateTime) > 2*time.Minute && s.ReqCnt > 2) {
			s.fundRule()
		}
	}
	s.Lock.Lock()
	if time.Since(s.LastGc) > s.GetGcInterval() && s.idleInstance.Len() > 0 {
		m2.Printf("GcPreSize %s,%d,%d,%d,%d,%d,%d,%d", s.MetaData.Key, s.idleInstance.Len(), s.IdleSize.Load(), s.OnWait.Load(), s.OnCreate.Load(), s.InstanceSize.Load(), s.GcTime/1000, s.GcCnt)
		s.LastGc = time.Now()
		cur := s.idleInstance.Front()
		for {
			next := cur.Next()
			element := cur
			instance := element.Value.(*m2.Instance)
			idleDuration := time.Now().Sub(instance.LastIdleTime)
			if idleDuration > s.GetIdleDurationBeforeGC() {
				m2.Printf("Idle duration: %dms, time excceed configured: %dms", idleDuration.Milliseconds(),
					s.GetIdleDurationBeforeGC().Milliseconds())
				s.idleInstance.Remove(element)
				m2.ReqLog("", s.MetaData.Key, "", "fastConsume10", idleDuration.Milliseconds())
				d.transferDelete(uuid.NewString(), instance.Slot.Id, instance.Id, instance.Meta.Key, "gc")
			}
			if next == nil {
				break
			}
			cur = next
		}
		m2.Printf("GcEndSize %s,%d,%d,%d,%d,%d", s.MetaData.Key, s.idleInstance.Len(), s.IdleSize.Load(), s.OnWait.Load(), s.OnCreate.Load(), s.InstanceSize.Load())
	}
	s.Lock.Unlock()
	if time.Since(s.LastKm) > 10*time.Second {
		s.LastKm = time.Now()
		// 准入条件，请求记录大于200，或创建时间大于5分钟且请求数大于1
		if s.ReqCnt >= 10 || (time.Since(s.CreateTime) > 2*time.Minute && s.ReqCnt > 2) {
			s.fundRule()
		}
	}
}

type Rule struct {
	// 1 周期型，每隔周期T1，在较短时间T2内发出N个请求，T2<<T1，T1>2*ColdStart
	// 2 均衡型，以某一水平持续发出请求,平均间隔周期<ColdStart
	Cate        int
	Cluster     *[2]*kmeans.Cluster
	PreWarmMs   int
	KeepAliveMs int64
	MaxMs       int
	GcSec       int
	Valid       bool
}

func (s *BaseScheduler) fundRule() {
	// 对目前的请求到达间隔进行二分类
	data := s.ITData[:]
	if s.ReqCnt < 200 {
		data = data[:s.ReqCnt]
	}
	cs := kmeans.PartitionV1(data)
	m2.Printf("partResult:%v,%v", cs[0], cs[1])
	A := cs[0]
	B := cs[1]
	rule := &Rule{
		Cate:        0,
		Cluster:     cs,
		PreWarmMs:   0,
		KeepAliveMs: 0,
		MaxMs:       0,
		GcSec:       40,
		Valid:       false,
	}
	// 1.初始化时间<80MS,冷启动时间少，执行时间少，资源浪费大的应用，可能有多个较大周期，
	//  此类应用初始化时间极少，核心目标是找到一个可以放心GC的时间间隔，不需要过多考虑预创建
	//  数据集1，2 前置条件判断
	if s.InitMs < 80 && s.AvgExec < 80 {
		// 1.2 集合B的Min>Gc时间，这一部分的数据是一定会冷启动的，此时我们需要找到一个时间节点提前GC，节约资源
		if B.Min > 40000 {
			// 集合A的Max<15秒，然后我们将GC时间调整为MaxA+5秒
			if A.Max < 20000 {
				rule.Valid = true
				rule.GcSec = int(A.Max)/1000 + 5
			} else if A.Min > 40000 {
				rule.Valid = true
				// 集合A的Min也大于40秒，说明是长周期单次执行类型，我们将GC策略调整为立即删除
				rule.GcSec = 0
			}
		} else if A.Max < 2000 && B.Min > 20000 && len(B.Points) > 4*len(A.Points) {
			rule.Valid = true
			// 集合BMin>20秒，但此时集合A的数量比较大，且间隔很短，此时提前GC也是有收益的
			rule.GcSec = int(A.Max)/1000 + 5
		}
	}
	if rule.Valid == true {
		m2.Printf("getRule:Cate:%d,Gc:%d,%s", rule.Cate, rule.GcSec, s.MetaData.Key)
	}
	s.Rule = rule
}