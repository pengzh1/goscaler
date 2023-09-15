package scaler

import (
	"context"
	"fmt"
	"github.com/AliyunContainerService/scaler/go/pkg/config"
	m2 "github.com/AliyunContainerService/scaler/go/pkg/model"
	platform_client2 "github.com/AliyunContainerService/scaler/go/pkg/platform_client"
	pb "github.com/AliyunContainerService/scaler/proto"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"sort"
	"sync"
	"time"
)

type Dispatcher struct {
	WaitCheckChan chan *CreateEvent
	DeleteChan    chan *DeleteEvent
	GcChan        chan *BaseScheduler
	ScalerMap     sync.Map
	Deleted       sync.Map
	rw            sync.RWMutex
	config        *config.Config
}

func RunDispatcher(ctx context.Context, config *config.Config) *Dispatcher {
	dispatcher := &Dispatcher{
		WaitCheckChan: make(chan *CreateEvent, 10240),
		DeleteChan:    make(chan *DeleteEvent, 10240),
		GcChan:        make(chan *BaseScheduler, 1024),
		ScalerMap:     sync.Map{},
		Deleted:       sync.Map{},
		config:        config,
	}
	for i := 0; i < 20; i++ {
		go dispatcher.runCreateHandler(ctx, config.ClientAddr, i)
	}
	for i := 0; i < 10; i++ {
		go dispatcher.runDeleteHandler(ctx, config.ClientAddr, i)
	}
	for i := 0; i < 10; i++ {
		go dispatcher.runGcChecker(ctx)
	}
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-tick.C:
				dispatcher.ScalerMap.Range(func(key, value any) bool {
					if value != nil {
						scaler := value.(*BaseScheduler)
						dispatcher.GcChan <- scaler
					}
					return true
				})
			}
		}
	}()
	return dispatcher
}

type CreateEvent struct {
	InstanceId string
	RequestId  string
	Scaler     *BaseScheduler
}
type IdleInstance struct {
	Instance *m2.Instance
	Source   int
}
type DeleteEvent struct {
	InstanceId string
	RequestId  string
	SlotId     string
	MetaKey    string
	Reason     string
}

func (d *Dispatcher) runCreateHandler(ctx context.Context, ClientAddr string, id int) {
	client, err := platform_client2.New(ClientAddr)
	if err != nil {
		log.Fatal().Msg("client init with error" + err.Error())
	}
	for {
		select {
		case checkEvent := <-d.WaitCheckChan:
			start := time.Now()
			d.checkWait(checkEvent.Scaler, checkEvent.RequestId, checkEvent.InstanceId, client.(*platform_client2.PlatformClient))
			m2.Printf("worker%d check runEnd %dus", id, time.Since(start).Microseconds())
		case <-ctx.Done():
			return
		}
	}
}
func (d *Dispatcher) runDeleteHandler(ctx context.Context, ClientAddr string, id int) {
	client, err := platform_client2.New(ClientAddr)
	if err != nil {
		log.Fatal().Msg("client init with error" + err.Error())
	}
	for {
		select {
		case delEvent := <-d.DeleteChan:
			start := time.Now()
			d.deleteSlot(delEvent.RequestId, delEvent.SlotId, delEvent.InstanceId, delEvent.MetaKey, delEvent.Reason, client.(*platform_client2.PlatformClient))
			m2.Printf("worker%d del runEnd %dus", id, time.Since(start).Microseconds())
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) runGcChecker(ctx context.Context) {
	for {
		select {
		case s := <-d.GcChan:
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
				continue
			} else if s.OnWait.Load() == 0 {
				// 2.onWait=0，检查idleChan是否已清空
			loop:
				for {
					select {
					case ret := <-s.IdleChan:
						s.IdleSize.Add(-1)
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
			m2.Printf("gcRunEnd%s,%dus", s.MetaData.Key, time.Since(start).Microseconds())
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) Fetch(instanceId string, requestId string, meta *m2.Meta) *m2.Instance {
	// 1.获取scaler
	scaler, _ := d.ScalerMap.Load(meta.Key)
	s := scaler.(*BaseScheduler)
	// 2.从idle中获取，有则直接返回
	instance := d.getFromIdle(s, requestId)
	if instance != nil {
		instance.Busy = true
		instance.LastStart = time.Now()
		return instance
	}
	createEvent := &CreateEvent{
		InstanceId: instanceId,
		RequestId:  requestId,
		Scaler:     s,
	}
	// 3.尝试从idleChan中获取，超时时间40ms
	s.OnWait.Add(1)
	defer s.OnWait.Add(-1)
	if s.InstanceSize.Load() < 1 {
		// 4.idle中获取不到，增加预期实例数，触发一次实例创建检查
		if s.OnCreate.Load() < s.OnWait.Load() {
			d.WaitCheckChan <- createEvent
		}
		// 5.等待从IdleChan中接收实例，接收到后减少预期实例数
		instanceRet := <-s.IdleChan
		s.IdleSize.Add(-1)
		instanceRet.Instance.Busy = true
		return instanceRet.Instance
	}
	tick := time.NewTimer(40 * time.Millisecond)
	select {
	case instanceRet := <-s.IdleChan:
		s.IdleSize.Add(-1)
		m2.Printf("fastReturn")
		instanceRet.Instance.Busy = true
		instanceRet.Instance.LastStart = time.Now()
		return instanceRet.Instance
	case <-tick.C:
		m2.Printf("waitTimeout")
		select {
		case instanceRet := <-s.IdleChan:
			s.IdleSize.Add(-1)
			m2.Printf("fastReturn")
			instanceRet.Instance.Busy = true
			instanceRet.Instance.LastStart = time.Now()
			return instanceRet.Instance
		default:
			// 4.idle中获取不到，增加预期实例数，触发一次实例创建检查
			if s.OnCreate.Load() < s.OnWait.Load() {
				d.WaitCheckChan <- createEvent
			}
			// 5.等待从IdleChan中接收实例，接收到后减少预期实例数
			instanceRet := <-s.IdleChan
			s.IdleSize.Add(-1)
			instanceRet.Instance.Busy = true
			return instanceRet.Instance
		}
	}
}

func (d *Dispatcher) Idle(instanceId, requestId, meta, reason string, needDelete bool) error {
	scaler, _ := d.ScalerMap.Load(meta)
	s := scaler.(*BaseScheduler)
	if v, ok := s.instances.Load(instanceId); ok {
		instance := v.(*m2.Instance)
		if needDelete {
			d.transferDelete(uuid.NewString(), instance.Slot.Id, instanceId, meta, "bad instance")
			return nil
		}
		if !instance.Busy {
			m2.ReqLog(requestId, s.MetaData.Key, instanceId, "AlreadyFree", -1)
			return nil
		}
		instance.Busy = false
		instance.LastIdleTime = time.Now()
		idleInstance := &m2.CreateRet{
			Instance: instance,
		}
		// onWait大于0时，直接推送IdleChan，否则保存到idleInstances链表
		if s.OnWait.Load() > 0 {
			m2.Printf("pushIdleChan")
			s.IdleChan <- idleInstance
			s.IdleSize.Add(1)
		} else {
			s.Lock.Lock()
			m2.Printf("pushFront")
			s.idleInstance.PushFront(instance)
			s.Lock.Unlock()
		}
	}
	return nil
}

func (d *Dispatcher) GetOrCreate(metaData *m2.Meta) Scaler {
	if s, ok := d.ScalerMap.Load(metaData.Key); ok {
		return s.(*BaseScheduler)
	}
	d.rw.Lock()
	if s, ok := d.ScalerMap.Load(metaData.Key); ok {
		d.rw.Unlock()
		return s.(*BaseScheduler)
	}
	m2.Printf("Create new scaler for app %s", metaData.Key)
	scheduler := NewBaseScheduler(metaData, d.config, d)
	d.ScalerMap.Store(metaData.Key, scheduler)
	d.rw.Unlock()
	return scheduler
}

func (d *Dispatcher) Get(meta string) (Scaler, bool) {
	v, ok := d.ScalerMap.Load(meta)
	return v.(*BaseScheduler), ok
}

func (d *Dispatcher) checkWait(s *BaseScheduler, reqId, instId string, client *platform_client2.PlatformClient) {
	// 1.onWait小于1，将idleChan中的实例都Destroy
	if s.OnWait.Load() < 1 {
		m2.Printf("quickCheck")
		for {
			select {
			case idle := <-s.IdleChan:
				s.IdleSize.Add(-1)
				if idle != nil {
					d.transferDelete(uuid.NewString(), idle.Instance.Slot.Id, idle.Instance.Id, s.MetaData.Key, "bad instance")
				}
				continue
			default:
				return
			}
		}
	}
	// 2.onWait小于OnCreate，或onCrete大于1000，本次不创建实例，直接返回
	if s.OnCreate.Load() > s.OnWait.Load() || s.OnCreate.Load() > 100 {
		m2.Printf("dropCreateEvent:" + s.MetaData.Key)
		return
	}
	// 3.onWait大于OnCreate，尝试从idle中获取, 获取到则往idleChan推送
	idle := &m2.CreateRet{}
	if it := d.getFromIdle(s, reqId); it != nil {
		m2.Printf("quickIdle")
		idle.Instance = it
		s.IdleChan <- idle
		s.IdleSize.Add(1)
		return
	}
	// 4.onWait大于OnCreate，idle也为空，则开启创建协程，增加OnCreate数量
	idx := time.Now().UnixNano()
	// 用于接收实例可用数量变化推送, 在不需要实例的情况下，直接快速销毁当前初始化中的实例
	cc := make(chan struct{}, 1)
	s.OnCreate.Add(1)
	s.CheckChan.Store(idx, cc)
	go func() {
		d.CreateNew(context.Background(), instId, reqId, s, client, cc, idx)
		s.OnCreate.Add(-1)
		s.CheckLock.Lock()
		s.CheckChan.Delete(idx)
		close(cc)
		s.CheckLock.Unlock()
	}()
}

func (d *Dispatcher) transferDelete(reqId, slotId, instanceId, metaKey, reason string) {
	event := &DeleteEvent{
		InstanceId: instanceId,
		RequestId:  reqId,
		SlotId:     slotId,
		MetaKey:    metaKey,
		Reason:     reason,
	}
	d.DeleteChan <- event
}

func (d *Dispatcher) deleteSlot(reqId, slotId, instanceId, metaKey, reason string, client *platform_client2.PlatformClient) {
	start := time.Now()
	if _, ok := d.Deleted.Load(slotId); ok {
		return
	}
	m2.ReqLog(reqId, metaKey, instanceId, "deleteInstanceStart", -1)
	if inst, ok := d.ScalerMap.Load(metaKey); ok {
		s := inst.(*BaseScheduler)
		if _, ok := s.instances.LoadAndDelete(instanceId); ok {
			s.InstanceSize.Add(-1)
		}

	}
	go func() {
		err := client.DestroySLot(context.Background(), reqId, slotId, reason)
		if err != nil {
			m2.Printf("deleteEnd" + err.Error())
		}
	}()
	d.Deleted.Store(slotId, true)
	m2.ReqLog(reqId, metaKey, instanceId, "deleteInstanceEnd", time.Since(start).Milliseconds())
}

func (d *Dispatcher) CreateNew(ctx context.Context, instanceId, requestId string,
	s *BaseScheduler, client *platform_client2.PlatformClient, check chan struct{}, id int64) {
	m2.Printf("AssignNew,%s,%s", requestId, s.MetaData.Key)
	resourceConfig := m2.SlotResourceConfig{
		ResourceConfig: pb.ResourceConfig{
			MemoryInMegabytes: s.MetaData.MemoryInMb,
		},
	}
	start := time.Now()
	var slot *m2.Slot
	var err error
	// 1. 创建Slot 100ms左右 TODO 低内存共享Slot,预创建Slot
	slot, err = d.createSlot(ctx, requestId, &resourceConfig, client)
	if err != nil {
		m2.Printf("createFailed:%s", fmt.Sprintf("create slot failed with: %s", err.Error()))
		return
	}
	// 2.经过100ms后，可能经过Idle，onWait已小于OnCreate，此时直接销毁Slot，return
	// 加锁避免各个线程都依次减少
	s.Lock.Lock()
	if d.getInitIndex(s, id)+1 > int(s.OnWait.Load()) {
		s.Lock.Unlock()
		m2.Printf("quickDel")
		go func() {
			m2.Printf("fastDelete:%s", s.MetaData.Key)
			d.transferDelete(uuid.NewString(), slot.Id, instanceId, s.MetaData.Key, "fastDelete")
			m2.ReqLog(requestId, s.MetaData.Key, instanceId, "fastConsume1", time.Since(start).Milliseconds())
		}()
		return
	}
	s.Lock.Unlock()
	m2.Printf("AssignInit,%s,%s", requestId, s.MetaData.Key)
	rMeta := &m2.Meta{
		Meta: pb.Meta{
			Key:           s.MetaData.Key,
			Runtime:       s.MetaData.Runtime,
			TimeoutInSecs: s.MetaData.TimeoutInSecs,
		},
	}
	// 3. 执行初始化
	initRec := make(chan *m2.Instance)
	result := &m2.CreateRet{}
	go d.init(initRec, requestId, instanceId, slot, rMeta, client)
	for {
		select {
		case instance := <-initRec:
			// 3.1 接收到已创建的实例，若此时onWait大于0，直接推送，否则直接销毁Slot
			if instance != nil {
				if s.OnWait.Load() < 1 {
					m2.Printf("quickIdle2")
					d.transferDelete(uuid.NewString(), slot.Id, instanceId, s.MetaData.Key, "fastDelete7")
					m2.ReqLog(requestId, s.MetaData.Key, instanceId, "fastConsume7", time.Since(start).Milliseconds())
					return
				}
				result.Instance = instance
				s.instances.Store(instance.Id, instance)
				s.InstanceSize.Add(1)
				s.IdleChan <- result
				s.IdleSize.Add(1)
				m2.Printf("sendSuc")
				return
			} else {
				return
			}
		case <-check:
			idx := d.getInitIndex(s, id) + 1
			m2.Printf("checkIdx %s,%d,%d", requestId, idx, s.OnWait.Load())
			// 3.2 接收到实例数变动消息，检查initIndex 是否大于onWait数量，大于则直接销毁当前实例
			if d.getInitIndex(s, id)+1 > int(s.OnWait.Load()) {
				d.transferDelete(uuid.NewString(), slot.Id, instanceId, s.MetaData.Key, "fastDelete4")
				m2.ReqLog(requestId, s.MetaData.Key, instanceId, "fastConsume4", time.Since(start).Milliseconds())
				// 此时init函数可能还会返回结果，做兜底处理
				go func() {
					select {
					case inst := <-initRec:
						if inst != nil {
							d.transferDelete(uuid.NewString(), slot.Id, instanceId, s.MetaData.Key, "fastDelete8")
							m2.ReqLog(requestId, s.MetaData.Key, instanceId, "fastConsume8", time.Since(start).Milliseconds())
						}
					}
				}()
				return
			}
		}
	}
}

func (d *Dispatcher) getFromIdle(s *BaseScheduler, reqId string) *m2.Instance {
	s.Lock.Lock()
	if element := s.idleInstance.Front(); element != nil {
		instance := s.idleInstance.Remove(element).(*m2.Instance)
		s.Lock.Unlock()
		instance.Busy = true
		if instance.LastIdleTime.Sub(instance.LastStart) > 0 {
			m2.ReqLog(reqId, s.MetaData.Key, instance.Id, "reuseConsume2", time.Since(instance.LastIdleTime).Milliseconds())
		}
		instance.LastStart = time.Now()
		m2.Printf("getFromIdleSuc")
		return instance
	}
	s.Lock.Unlock()
	return nil
}

func (d *Dispatcher) createSlot(ctx context.Context, requestId string, slotResourceConfig *m2.SlotResourceConfig, client *platform_client2.PlatformClient) (*m2.Slot, error) {
	return client.CreateSlot(ctx, requestId, slotResourceConfig)
}

func (d *Dispatcher) getInitIndex(s *BaseScheduler, idx int64) int {
	id := 0
	s.CheckChan.Range(func(key, value any) bool {
		if key.(int64) < idx {
			id += 1
		}
		return true
	})
	return id
}

func (d *Dispatcher) init(initRec chan *m2.Instance, requestId string, instId string,
	slot *m2.Slot, rMeta *m2.Meta, client *platform_client2.PlatformClient) {
	instance, err := client.Init(context.Background(), requestId, instId, slot, rMeta)
	if err != nil {
		errorMessage := fmt.Sprintf("create instance failed with: %s", err.Error())
		m2.Printf(errorMessage)
		close(initRec)
		return
	}
	select {
	case initRec <- instance:
		m2.Printf("initSendSuccess")
		close(initRec)
	}
}

func (d *Dispatcher) GC(s *BaseScheduler) {
	s.Lock.Lock()
	if time.Since(s.LastGc) > s.GetGcInterval() && s.idleInstance.Len() > 0 {
		m2.Printf("GcPreSize %s,%d,%d,%d,%d,%d", s.MetaData.Key, s.idleInstance.Len(), s.IdleSize.Load(), s.OnWait.Load(), s.OnCreate.Load(), s.InstanceSize.Load())
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
}