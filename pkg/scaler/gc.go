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
	tick := time.NewTicker(130 * time.Millisecond)
	for {
		select {
		case <-tick.C:
			d.fastTermCheck()
			ss := make([]*BaseScheduler, 0, d.ScalerSize.Load())
			d.ScalerMap.Range(func(key, value any) bool {
				if value != nil {
					ss = append(ss, value.(*BaseScheduler))
				}
				return true
			})
			m2.Printf("gcStart")
			for _, s := range ss {
				d.runGc(s)
			}
			m2.Printf("gcEnd")
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
			if len(arr) > 0 {
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
		if s.ReqCnt >= 10 || (time.Since(s.CreateTime) > 2*time.Minute) || (time.Since(s.CreateTime) > 1*time.Minute && s.ReqCnt > 1 && s.InitMs < 80 && s.AvgExec < 80) {
			s.FundRule()
		}
	}
	s.Lock.Lock()
	if time.Since(s.LastGc) > s.GetGcInterval() && s.idleInstance.Len() > 0 {
		preSize := s.InstanceSize.Load()
		delta := 0
		m2.Printf("GcPreSize %s,%d,%d,%d,%d,%d,%d,%d", s.MetaData.Key, s.idleInstance.Len(), s.IdleSize.Load(), s.OnWait.Load(), s.OnCreate.Load(), s.InstanceSize.Load(), s.GcTime/1000, s.GcCnt)
		s.LastGc = time.Now()
		cur := s.idleInstance.Front()
		for {
			next := cur.Next()
			element := cur
			instance := element.Value.(*m2.Instance)
			idleDuration := time.Now().Sub(instance.LastIdleTime)
			if idleDuration > s.GetIdleDurationBeforeGC(instance, int(preSize)-delta) {
				if instance.KeepAliveMs > 0 && instance.LastStart.Sub(instance.LastIdleTime).Milliseconds() == 0 {
					m2.Printf("ruleFastDelete:%s", s.MetaData.Key)
					//s.Rule.Valid = false
				}
				m2.Printf("Idle duration: %dms, time excceed configured: %dms", idleDuration.Milliseconds(),
					s.GetIdleDurationBeforeGC(instance, int(preSize)-delta).Milliseconds())
				s.idleInstance.Remove(element)
				delta += 1
				m2.ReqLog("", s.MetaData.Key, "", "fastConsume10", idleDuration.Milliseconds())
				d.transferDelete(uuid.NewString(), instance.Slot.Id, instance.Id, instance.Meta.Key, "gc")
			}
			if next == nil {
				break
			}
			cur = next
		}
		m2.Printf("GcEndSize %s,%d,%d,%d,%d,%d", s.MetaData.Key, s.idleInstance.Len(), s.IdleSize.Load(), s.OnWait.Load(), s.OnCreate.Load(), s.InstanceSize.Load())
		if preSize > 0 && preSize == int32(delta) && !s.d.fastTerm() {
			if s.Rule.Valid && s.Rule.PreWarmMs > 0 && s.Rule.KeepAliveMs > 0 && s.Rule.GcMill > 0 {
				preWarm := s.Rule.PreWarmMs
				preWarm -= time.Now().UnixMilli() - s.LastTime
				m2.Printf("preWarmAfter:%s,%d,%d,%d", s.MetaData.Key, preWarm, s.Rule.KeepAliveMs, s.Rule.PreCreateCnt)
				if preWarm > 0 {
					for i := 0; i < s.Rule.PreCreateCnt; i++ {

						createEvent := &CreateEvent{
							InstanceId: uuid.NewString(),
							RequestId:  s.Rule.Cate + uuid.NewString(),
							Scaler:     s,
							PreWarm:    true,
						}
						go func() {
							tick := time.NewTimer(time.Duration(preWarm) * time.Millisecond)
							select {
							case <-tick.C:
								d.WaitCheckChan <- createEvent
								tick.Stop()
							}
						}()
					}
				}
			}
		}
	}
	s.Lock.Unlock()
	if time.Since(s.LastKm) > 10*time.Second {
		s.LastKm = time.Now()
		// 准入条件，请求记录大于200，或创建时间大于5分钟且请求数大于1
		if s.ReqCnt >= 10 || (time.Since(s.CreateTime) > 2*time.Minute && s.ReqCnt > 2) {
			s.FundRule()
		}
	}
}

type Rule struct {
	// 1 周期型，每隔周期T1，在较短时间T2内发出N个请求，T2<<T1，T1>2*ColdStart
	// 2 均衡型，以某一水平持续发出请求,平均间隔周期<ColdStart
	Cate         string
	Cluster      *[2]*kmeans.Cluster
	PreWarmMs    int64
	KeepAliveMs  int64
	MaxMs        int
	GcMill       int
	GcSingleMill int
	Valid        bool
	PreCreateCnt int
}

func (s *BaseScheduler) FundRule() {
	if s.d.fastTerm() {
		return
	}
	// 对目前的请求到达间隔进行二分类
	rawData := s.ITData[:]
	maxIt := int32(0)
	sumIt := int32(0)
	for _, n := range rawData {
		if n > maxIt {
			maxIt = n
		}
		sumIt += n
	}
	var data []int32
	// 取最大周期五倍的数据
	dur := 5 * maxIt
	if dur < sumIt && sumIt > 70000 {
		if dur < 70000 {
			dur = 70000
		}
		req := s.ReqCnt - 2
		start := req - 200
		if start < 0 {
			start = -1
		}
		for dur > 0 && req > start {
			data = append(data, rawData[req%len(rawData)])
			dur -= rawData[req%len(rawData)]
			req -= 1
		}
	} else {
		if s.ReqCnt < 200 {
			rawData = rawData[:s.ReqCnt-1]
		}
		data = rawData
	}
	rule := &Rule{
		Cate:        "",
		Cluster:     nil,
		PreWarmMs:   0,
		KeepAliveMs: 0,
		MaxMs:       0,
		GcMill:      40000,
		Valid:       false,
	}
	if len(data) > 1 {
		cs := kmeans.PartitionV1(data)
		rule.Cluster = cs
		m2.Printf("partResult:%s,%v,%v,%v,%v", s.MetaData.Key, cs[0], cs[1], data)
		A := cs[0]
		B := cs[1]
		// 1.初始化时间<80MS,冷启动时间少，执行时间少，资源浪费大的应用，可能有多个较大周期，
		//  此类应用初始化时间极少，核心目标是找到一个可以放心GC的时间间隔，不需要过多考虑预创建
		if s.InitMs < 80 && s.AvgExec < 80 {
			if time.Now().UnixMilli()-s.LastTime > int64(B.Max) {
				s.Rule = rule
				return
			}
			if A.Min > 10000 && B.Max-A.Min < A.Min/5 {
				// 固定周期单次执行类型，执行完毕后，立即销毁，发起一个定时实例创建
				// 延迟时间：A.Min-1S, 存活时间B.MAX-A.Min+3S
				rule.PreWarmMs = int64(A.Min - 1000)
				rule.KeepAliveMs = int64(B.Max - A.Min + 3000)
				rule.GcMill = 0
				rule.Cate = "single"
				rule.Valid = true
				rule.PreCreateCnt = 1
			} else if B.Max-B.Min < B.Min/10 && B.Min > 15000 && A.Max < 2000 {
				// 固定周期多次执行类型，执行完毕后，等待一段时间后销毁，实例清零时发起一个定时实例创建
				// 延迟时间：B.Min-1S, 存活时间B.Max-B.Min+3S
				rule.PreWarmMs = int64(B.Min - 1200)
				rule.KeepAliveMs = int64(B.Max - B.Min + 3000)
				if rule.KeepAliveMs > 40000 {
					rule.KeepAliveMs = 40000
				}
				rule.GcMill = int(A.Max + 1000)
				if A.Max < 500 {
					rule.GcMill = int(A.Max * 2)
				}
				if A.Max < 100 {
					rule.GcMill = int(A.Max + 200)
				}
				rule.Cate = "multi"
				rule.Valid = true
				rule.PreCreateCnt = 1
				m2.Printf("%sMaxWorking%d", s.MetaData.Key, s.LastMaxWorking)
				if s.LastMaxWorking > 1 {
					rule.PreCreateCnt = 2
				}
				if s.LastMaxWorking > 4 {
					rule.PreCreateCnt = 3
				}
			} else {
				// 1.2 集合B的Min>Gc时间，这一部分的数据是一定会冷启动的，此时我们可以找到一个时间节点提前GC，节约资源
				if B.Min > 40000 {
					// 集合A的Max<15秒，然后我们将GC时间调整为MaxA+2秒
					if A.Max < 20000 {
						rule.Valid = true
						rule.GcMill = int(A.Max + 2000)
						if A.Max < 600 {
							rule.GcMill = int(A.Max * 2)
						}
						if A.Max < 100 {
							rule.GcMill = int(A.Max + 200)
						}
					} else if A.Min > 40000 {
						rule.Valid = true
						// 集合A的Min也大于40秒，说明是长周期单次执行类型，我们将GC策略调整为立即删除
						rule.GcMill = 0
					}
				} else if (A.Max < 1000 || (A.Max < 3000 && A.Center < 1000)) && B.Min > 20000 && len(A.Points) > 5*len(B.Points) {
					rule.Valid = true
					// 集合BMin>20秒，但此时集合A的数量比较大，且间隔很短，此时提前GC也是有收益的
					rule.GcMill = int(A.Max + 1000)
					if A.Max < 500 {
						rule.GcMill = int(A.Max * 2)
					}
					if A.Max < 100 {
						rule.GcMill = int(A.Max + 200)
					}
				}
			}
		}
		//  长初始化周期类型
		if s.InitMs > 1000 {
			if time.Now().UnixMilli()-s.LastTime > int64(B.Max) {
				s.Rule = rule
				return
			}
			if A.Min > 25000 && B.Max-A.Min < A.Min/5 {
				// 固定周期单次执行类型，执行完毕后，立即销毁，发起一个定时实例创建
				// 延迟时间：A.Min-2S-InitTime, 存活时间A.MAX-A.Min+2S
				rule.PreWarmMs = int64(A.Min - 2000 - s.InitMs)
				rule.KeepAliveMs = int64(B.Max - A.Min + 4000)
				rule.GcMill = 0
				rule.Cate = "single"
				rule.Valid = true
				rule.PreCreateCnt = 1
			} else if B.Max-B.Min < B.Min/10 && B.Min > 20000 && A.Max < 2000 && s.AvgExec > 3 {
				// 固定周期多次执行类型，实例全部销毁后，发起N个实例创建，N=lastMaxWorking
				// 延迟时间：B.Min-2S, 存活时间B.Max-B.Min+2S
				rule.PreWarmMs = int64(B.Min - 2000 - s.InitMs)
				rule.KeepAliveMs = int64(B.Max - B.Min + 4000)
				if rule.KeepAliveMs > 40000 {
					rule.KeepAliveMs = 40000
				}
				rule.GcMill = int(A.Max + 2000)
				rule.Cate = "multi"
				rule.Valid = true
				rule.PreCreateCnt = int(s.LastMaxWorking)
				if s.LastMaxWorking == 0 {
					rule.PreCreateCnt = 1
				}
			} else {
				// 超短执行时间的应用，且间隔周期不固定的应用，预留实例
				if s.InitMs > 5000 && s.AvgExec < 5 && s.MetaData.MemoryInMb < 2000 && A.Center > 10 {
					rule.Valid = true
					rule.GcMill = int(B.Max + 10000)
					rule.GcSingleMill = int(B.Max + 60000)
					if rule.GcSingleMill < 300000 {
						rule.GcSingleMill = 300000
					}
					// 集合B的Min>Gc时间，这一部分的数据是一定会冷启动的，此时我们可以找到一个时间节点提前GC，节约资源
				} else if B.Min > 40000 {
					// 集合A的Max<15秒，然后我们将GC时间调整为MaxA+3秒
					if A.Max < 20000 {
						rule.Valid = true
						rule.GcMill = int(A.Max) + 3000
					} else if A.Min > 40000 {
						rule.Valid = true
						// 集合A的Min也大于40秒，说明是长周期单次执行类型，我们将GC策略调整为立即删除
						rule.GcMill = 0
					}
				} else if (A.Max < 1000 || (A.Max < 3000 && A.Center < 1000)) && B.Min > 20000 && len(A.Points) > 5*len(B.Points) && s.MetaData.MemoryInMb > 2048 {
					rule.Valid = true
					// 集合BMin>20秒，但此时集合A的数量比较大，且间隔很短，此时提前GC也是有收益的
					rule.GcMill = int(A.Max + 5000)
				}
			}
		}
		cs[0].Points = nil
		cs[1].Points = nil
		if rule.Valid == true {
			m2.Printf("getRule:%s,%v,%v", s.MetaData.Key, rule, rule.Cluster)
		}
	}
	s.Rule = rule
}
