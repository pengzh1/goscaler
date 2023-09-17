package scaler

import (
	"container/list"
	"context"
	"fmt"
	"github.com/AliyunContainerService/scaler/go/pkg/kmeans"
	m2 "github.com/AliyunContainerService/scaler/go/pkg/model"
	platform_client2 "github.com/AliyunContainerService/scaler/go/pkg/platform_client"
	pb "github.com/AliyunContainerService/scaler/proto"
	"github.com/rs/zerolog/log"
	"sync"
	"sync/atomic"
	"time"
)

type SlotPool struct {
	MemInMb    uint64
	Lock       sync.Mutex
	scaler     sync.Map
	OnWait     atomic.Int32
	OnCreate   atomic.Int32
	CreateTime time.Time
	ReqCnt     int
	EndCnt     int
	ITLock     sync.Mutex
	LastTime   int64
	ITTime     *[200]int64
	ITData     *[200]int32
	LastExec   *[20]int32
	CacheChan  chan *m2.Slot
	IdleSize   atomic.Int32
	Rule       *Rule
	AppCnt     int
	SumInit    int
	SumExec    int
	Conf       *m2.SlotResourceConfig
	idleSlot   *list.List
	client     *platform_client2.PlatformClient
}

func NewSlotPool(memInMb uint64, addr string) *SlotPool {
	pool := &SlotPool{
		MemInMb:    memInMb,
		scaler:     sync.Map{},
		OnWait:     atomic.Int32{},
		OnCreate:   atomic.Int32{},
		CreateTime: time.Time{},
		IdleSize:   atomic.Int32{},
		ReqCnt:     0,
		EndCnt:     0,
		LastTime:   0,
		ITTime:     new([200]int64),
		ITData:     new([200]int32),
		LastExec:   new([20]int32),
		CacheChan:  make(chan *m2.Slot),
		Rule:       &Rule{},
		AppCnt:     0,
		SumInit:    0,
		SumExec:    0,
		Conf: &m2.SlotResourceConfig{
			ResourceConfig: pb.ResourceConfig{
				MemoryInMegabytes: memInMb,
			},
		},
		idleSlot: list.New(),
	}
	client, err := platform_client2.New(addr)
	if err != nil {
		log.Fatal().Msg("client init with error" + err.Error())
	}
	pool.client = client.(*platform_client2.PlatformClient)
	m2.Printf("NewSlotPool for mem: %s is created", memInMb)
	go func() {
		tick := time.NewTicker(5 * time.Second)
		tick2 := time.NewTicker(50 * time.Millisecond)
		for {
			select {
			case <-tick.C:
				// 准入条件，请求记录大于200，或创建时间大于5分钟且请求数大于1
				if pool.ReqCnt >= 10 || (time.Since(pool.CreateTime) > 2*time.Minute && pool.EndCnt > 2) {
					pool.fundRule()
				}
			case <-tick2.C:
				pool.check()
			}
		}
	}()
	return pool
}

func (s *SlotPool) ReqRecord(bs *BaseScheduler) {
	start := time.Now()
	s.ITLock.Lock()
	if _, ok := s.scaler.Load(bs.MetaData.Key); !ok {
		s.scaler.Store(bs.MetaData.Key, bs)
		s.AppCnt += 1
	}
	if s.LastTime == 0 {
		s.LastTime = start.UnixMilli()
	} else {
		it := int32(start.UnixMilli() - s.LastTime)
		s.ITTime[(s.ReqCnt-1)%len(s.ITTime)] = start.UnixMilli()
		s.ITData[(s.ReqCnt-1)%len(s.ITTime)] = it
		if s.Rule.Valid {
			// IT逃逸后设置规则无效
			if kmeans.Escape(it, s.Rule.Cluster[0]) && kmeans.Escape(it, s.Rule.Cluster[1]) {
				s.Rule.Valid = false
				m2.Printf("ruleSlotInvalid:%d,%d", s.MemInMb, it)
			}
		}
		s.LastTime = start.UnixMilli()
	}
	s.ReqCnt += 1
	s.ITLock.Unlock()
	return
}

func (s *SlotPool) EndRecord(dur int32) {
	s.ITLock.Lock()
	s.EndCnt += 1
	s.LastExec[(s.EndCnt-1)%len(s.LastExec)] = dur
	s.SumExec += int(dur)
	s.ITLock.Unlock()
}

func (s *SlotPool) getFromIdle(reqId string) *m2.Slot {
	s.Lock.Lock()
	if element := s.idleSlot.Front(); element != nil {
		slot := s.idleSlot.Remove(element).(*m2.Slot)
		s.Lock.Unlock()
		return slot
	}
	s.Lock.Unlock()
	return nil
}

func (s *SlotPool) check() {
	if s.OnWait.Load() == 0 {
		s.Lock.Lock()
		defer s.Lock.Unlock()
		var retain []*m2.Slot
		// 2.onWait=0，检查idleChan是否已清空
	loop:
		for {
			select {
			case ret := <-s.CacheChan:
				s.IdleSize.Add(-1)
				m2.ReqLog("", "", "", "fastConsume21", time.Since(ret.CreateTime).Milliseconds())
				if time.Now().UnixMilli() > ret.GcMs {
					go func() {
						_ = s.client.DestroySLot(context.Background(), "", ret.Id, "cacheTimeout3")
						m2.Printf("cacheSendTimeout3:%d", s.MemInMb)
					}()
				} else {
					retain = append(retain, ret)
				}
			default:
				break loop
			}
		}
		for _, c := range retain {
			s.IdleSize.Add(1)
			s.CacheChan <- c
		}
	}
}

func (s *SlotPool) Fetch(ctx context.Context, reqId string, bs *BaseScheduler, client *platform_client2.PlatformClient) *m2.Slot {
	s.OnWait.Add(1)
	defer s.OnWait.Add(-1)
	if s.MemInMb > 1024 {
		slot, _ := client.CreateSlot(ctx, reqId, s.Conf)
		return slot
	}
	for s.OnCreate.Load()+s.IdleSize.Load() < s.OnWait.Load() {
		s.OnCreate.Add(1)
		go func() {
			slot, err := client.CreateSlot(ctx, reqId, s.Conf)
			if err != nil {
				m2.Printf("createFailed:%s", fmt.Sprintf("create slot failed with: %s", err.Error()))
				s.OnCreate.Add(-1)
				return
			}
			slot.GcMs = time.Now().UnixMilli() + 50
			m2.Printf("cacheSendSuc:%d", s.MemInMb)
			s.CacheChan <- slot
			s.IdleSize.Add(1)
			s.OnCreate.Add(-1)
		}()
	}
	if s.Rule.Valid {
		s.Lock.Lock()
		for s.OnCreate.Load()+s.IdleSize.Load() < s.OnWait.Load()+int32(s.Rule.PreCreateCnt) {
			s.OnCreate.Add(1)
			go func() {
				slot, err := client.CreateSlot(ctx, reqId, s.Conf)
				if err != nil {
					m2.Printf("createFailed:%s", fmt.Sprintf("create slot failed with: %s", err.Error()))
					s.OnCreate.Add(-1)
					return
				}
				slot.GcMs = time.Now().UnixMilli() + s.Rule.KeepAliveMs
				m2.Printf("cacheSendSuc2:%d", s.MemInMb)
				s.CacheChan <- slot
				s.IdleSize.Add(1)
				s.OnCreate.Add(-1)
			}()
		}
		s.Lock.Unlock()
	}
	sl := <-s.CacheChan
	s.IdleSize.Add(-1)
	return sl
}

func (s *SlotPool) Return(slot *m2.Slot, reqId string, bs *BaseScheduler, client *platform_client2.PlatformClient) {
	tick := time.NewTimer(40 * time.Millisecond)
	select {
	case s.CacheChan <- slot:
		s.IdleSize.Add(1)
		m2.Printf("returnSendSuc2:%d", s.MemInMb)
	case <-tick.C:
		_ = client.DestroySLot(context.Background(), reqId, slot.Id, "cacheTimeout2")
		m2.Printf("cacheSendTimeout2:%d", s.MemInMb)
	}
}

func (s *SlotPool) fundRule() {
	// 对目前的请求到达间隔进行二分类
	data := s.ITData[:]
	if s.ReqCnt < 200 {
		data = data[:s.ReqCnt-1]
	}
	cs := kmeans.PartitionV2(data)
	m2.Printf("partSlotResult:%d,%v,%v,%v", s.MemInMb, cs[0], cs[1], data)
	A := cs[0]
	B := cs[1]
	rule := &Rule{
		Cate:        "",
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
	sumInit := 0
	s.scaler.Range(func(key, value any) bool {
		sumInit += int(value.(*BaseScheduler).InitMs)
		return true
	})
	if sumInit/s.AppCnt < 80 && s.SumExec/s.EndCnt < 80 {
		if time.Now().UnixMilli()-s.LastTime > int64(B.Max) {
			s.Rule = rule
			return
		}
		// 周期性burst流量，有请求到来时，可用Slot数+创建中数=等待数+2
		if A.Max < 1000 || (A.Max < 3000 && A.Center < 1000) && B.Min > 10000 {
			rule.Valid = true
			rule.PreCreateCnt = 2
			if 100/A.Max > 3 {
				rule.PreCreateCnt = int(100/A.Max) - 1
			}
			rule.KeepAliveMs = int64(A.Max + 200)
		}
	}
	if rule.Valid == true {
		m2.Printf("getSlotRule:Cate:%s,%d,%d,%d", rule.Cate, rule.PreCreateCnt, rule.KeepAliveMs, s.MemInMb)
	}
	s.Rule = rule
}
