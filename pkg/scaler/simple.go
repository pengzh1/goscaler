/*
Copyright 2023 The Alibaba Cloud Serverless Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scaler

import (
	"container/list"
	"context"
	"fmt"
	"github.com/AliyunContainerService/scaler/go/pkg/config"
	m2 "github.com/AliyunContainerService/scaler/go/pkg/model"
	platform_client2 "github.com/AliyunContainerService/scaler/go/pkg/platform_client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/AliyunContainerService/scaler/proto"
	"github.com/google/uuid"
)

type Simple struct {
	config         *config.Config
	MetaData       *m2.Meta
	platformClient platform_client2.Client
	mu             sync.Mutex
	wg             sync.WaitGroup
	instances      map[string]*m2.Instance
	idleInstance   *list.List
	Cnt            int64
	InitDuration   time.Duration
	Exec           *[5]int
	OnInit         int32
	UseTime        *[7200]uint16
	IdleChan       chan *m2.CreateRet
	Deleted        sync.Map
}

func New(metaData *m2.Meta, config *config.Config) Scaler {
	client, err := platform_client2.New(config.ClientAddr)
	if err != nil {
		log.Fatalf("client init with error: %s", err.Error())
	}
	scheduler := &Simple{
		config:         config,
		MetaData:       metaData,
		platformClient: client,
		mu:             sync.Mutex{},
		wg:             sync.WaitGroup{},
		instances:      make(map[string]*m2.Instance),
		idleInstance:   list.New(),
		Exec:           &[5]int{},
		OnInit:         0,
		UseTime:        &[7200]uint16{},
		IdleChan:       make(chan *m2.CreateRet),
	}
	m2.Printf("New scaler for app: %s is created", metaData.Key)
	scheduler.wg.Add(1)
	go func() {
		defer scheduler.wg.Done()
		scheduler.gcLoop()
		m2.Printf("gc loop for app: %s is stoped", metaData.Key)
	}()

	return scheduler
}

func (s *Simple) Assign(ctx context.Context, request *pb.AssignRequest) (*pb.AssignReply, error) {
	start := time.Now()
	instanceId := uuid.New().String()
	if m2.Dev {
		defer func() {
			m2.Printf("AssignEnd,%s,%s,%s,cost %dms", request.RequestId, request.MetaData.Key, instanceId, time.Since(start).Milliseconds())
		}()
		m2.Printf("AssignStart,%s,%s", request.RequestId, request.MetaData.Key)
	}
	s.mu.Lock()
	if element := s.idleInstance.Front(); element != nil {
		instance := element.Value.(*m2.Instance)
		instance.Busy = true
		s.idleInstance.Remove(element)
		instanceId = instance.Id
		s.mu.Unlock()
		m2.Printf("AssignReuse,%s,%s,%s reused", request.RequestId, request.MetaData.Key, instance.Id)

		if instance.LastIdleTime != instance.LastStart {
			m2.Printf("reuseConsume,%s,%d", s.MetaData.Key, time.Since(instance.LastIdleTime).Milliseconds())
		}
		instance.LastStart = time.Now()
		return &pb.AssignReply{
			Status: pb.Status_Ok,
			Assigment: &pb.Assignment{
				RequestId:  request.RequestId,
				MetaKey:    instance.Meta.Key,
				InstanceId: instance.Id,
			},
			ErrorMessage: nil,
		}, nil
	}
	//Create new Instance
	m2.Printf("AssignNew,%s,%s", request.RequestId, request.MetaData.Key)
	creatStart := time.Now()
	atomic.AddInt32(&s.OnInit, 1)
	go s.CreateNew(instanceId, request.RequestId, s.MetaData)
	var instance *m2.Instance
	s.mu.Unlock()
	select {
	case createRet := <-s.IdleChan:
		atomic.AddInt32(&s.OnInit, -1)
		if createRet.Err != nil {
			atomic.AddInt32(&s.OnInit, -1)
			return nil, status.Errorf(codes.Internal, "createErr")
		}
		instance = createRet.Instance
	}
	instanceId = instance.Id
	if instance.LastIdleTime != instance.LastStart {
		m2.Printf("waitConsume,%s,%d", s.MetaData.Key, time.Since(creatStart).Milliseconds())
	}
	//add new instance
	s.mu.Lock()
	instance.Busy = true
	if instance.LastIdleTime != instance.LastStart {
		m2.Printf("reuseConsume,%s,%d", s.MetaData.Key, time.Since(instance.LastIdleTime).Milliseconds())
	}
	instance.LastStart = time.Now()
	s.mu.Unlock()
	m2.Printf("AssignCreated,%s,%s,%dms", request.RequestId, instance.Meta.Key, time.Since(creatStart).Milliseconds())
	return &pb.AssignReply{
		Status: pb.Status_Ok,
		Assigment: &pb.Assignment{
			RequestId:  request.RequestId,
			MetaKey:    instance.Meta.Key,
			InstanceId: instance.Id,
		},
		ErrorMessage: nil,
	}, nil
}

func (s *Simple) Idle(ctx context.Context, request *pb.IdleRequest) (*pb.IdleReply, error) {
	if request.Assigment == nil {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("assignment is nil"))
	}
	reply := &pb.IdleReply{
		Status:       pb.Status_Ok,
		ErrorMessage: nil,
	}
	start := time.Now()
	instanceId := request.Assigment.InstanceId
	if m2.Dev {
		defer func() {
			m2.Printf("IdleEnd,%s,%s,cost %dus", request.Assigment.RequestId, request.Assigment.MetaKey, time.Since(start).Microseconds())
		}()
		m2.Printf("Idle, request id: %s", request.Assigment.RequestId)
	}
	needDestroy := false
	slotId := ""
	if request.Result != nil && request.Result.NeedDestroy != nil && *request.Result.NeedDestroy {
		needDestroy = true
	}
	defer func() {
		if needDestroy {
			s.deleteSlot(ctx, request.Assigment.RequestId, slotId, instanceId, request.Assigment.MetaKey, "bad instance")
		}
	}()
	m2.Printf("IdleStart,%s,%s,", request.Assigment.RequestId, request.Assigment.MetaKey)
	s.mu.Lock()
	if instance := s.instances[instanceId]; instance != nil {
		slotId = instance.Slot.Id
		instance.LastIdleTime = time.Now()
		if needDestroy {
			delete(s.instances, instanceId)
			s.mu.Unlock()
			m2.Printf("request id %s, instance %s need be destroy", request.Assigment.RequestId, instanceId)
			return reply, nil
		}

		if instance.Busy == false {
			s.mu.Unlock()
			m2.Printf("request id %s, instance %s already freed", request.Assigment.RequestId, instanceId)
			return reply, nil
		}
		instance.Busy = false
		s.mu.Unlock()
		select {
		case s.IdleChan <- &m2.CreateRet{Instance: instance}:
			m2.Printf("reuseByChan %s,%s", instanceId, s.MetaData.Key)
		default:
			m2.Printf("push2Idle %s,%s", instanceId, s.MetaData.Key)
			s.mu.Lock()
			s.idleInstance.PushFront(instance)
			s.mu.Unlock()
		}
		return &pb.IdleReply{
			Status:       pb.Status_Ok,
			ErrorMessage: nil,
		}, nil
	} else {
		s.mu.Unlock()
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("request id %s, instance %s not found", request.Assigment.RequestId, instanceId))
	}
}

func (s *Simple) deleteSlot(ctx context.Context, requestId, slotId, instanceId, metaKey, reason string) {
	m2.Printf("start delete Instance %s (Slot: %s) of app: %s", instanceId, slotId, metaKey)
	if _, ok := s.Deleted.Load(slotId); ok {
		return
	}
	if err := s.platformClient.DestroySLot(ctx, requestId, slotId, reason); err != nil {
		m2.Printf("delete Instance %s (Slot: %s) of app: %s failed with: %s", instanceId, slotId, metaKey, err.Error())
	}
	s.Deleted.Store(slotId, true)
	m2.Printf("delete Instance %s (Slot: %s) of app: %s succ", instanceId, slotId, metaKey)
}

func (s *Simple) gcLoop() {
	m2.Printf("gc loop for app: %s is started", s.MetaData.Key)
	ticker := time.NewTicker(1000 * time.Millisecond)
	lastGc := time.Now()
	//lastUsageCheck := time.Now()
	for range ticker.C {
		for {
			s.mu.Lock()
			avgExec := float32(0)
			m2.Printf("GcPreSize %s,%d,%d,%f,%v", s.MetaData.Key, len(s.instances), s.idleInstance.Len(), avgExec, s.Exec)
			if time.Since(lastGc) > s.config.GetGcInterval(float32(s.InitDuration), avgExec) && s.idleInstance.Len() > 0 {
				lastGc = time.Now()
				cur := s.idleInstance.Front()
				for {
					next := cur.Next()
					element := cur
					instance := element.Value.(*m2.Instance)
					idleDuration := time.Now().Sub(instance.LastIdleTime)
					if idleDuration > s.config.GetIdleDurationBeforeGC(float32(s.InitDuration), avgExec, s.UseTime, s.MetaData) {
						m2.Printf("Idle duration: %dms, time excceed configured: %dms", idleDuration.Milliseconds(),
							s.config.GetIdleDurationBeforeGC(float32(s.InitDuration), avgExec, s.UseTime, s.MetaData).Milliseconds())
						//need GC
						s.idleInstance.Remove(element)
						delete(s.instances, instance.Id)
						go func() {
							reason := fmt.Sprintf("Idle duration: %fs, excceed configured duration: %fs", idleDuration.Seconds(), s.config.IdleDurationBeforeGC.Seconds())
							ctx := context.Background()
							ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
							defer cancel()
							s.deleteSlot(ctx, uuid.NewString(), instance.Slot.Id, instance.Id, instance.Meta.Key, reason)
							m2.Printf("gcConsume,%s,%d", s.MetaData.Key, idleDuration.Milliseconds())
						}()
					}
					if next == nil {
						break
					}
					cur = next
				}
			}
			m2.Printf("GcEndSize %s,%d,%d", s.MetaData.Key, len(s.instances), s.idleInstance.Len())
			s.mu.Unlock()
			break
		}
	}
}

func (s *Simple) GetAvgExec() float32 {
	if s.Cnt < 1 {
		return 0
	}
	execHis := float32(0)
	cnt := float32(0)
	for exec := range s.Exec {
		if exec > 0 {
			execHis += float32(exec)
			cnt += 1
		}
	}
	avg := execHis / cnt
	cur := time.Now()
	for _, v := range s.instances {
		if v.Busy {
			duration := float32(cur.Sub(v.LastStart).Milliseconds())
			if duration > avg {
				execHis += duration * 1.5
				cnt += 1
			}
		}
	}
	m2.Printf("avgExecCompute %d,%f,%f", s.Cnt, execHis, cnt)
	return execHis / cnt
}

func (s *Simple) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{
		TotalInstance:     len(s.instances),
		TotalIdleInstance: s.idleInstance.Len(),
	}
}

func (s *Simple) CreateNew(instanceId string, requestId string, meta *m2.Meta) {
	ctx := context.Background()
	result := &m2.CreateRet{
		Instance: nil,
		Err:      nil,
		Msg:      "",
	}
	//Create new Instance
	m2.Printf("AssignNew,%s,%s", requestId, meta.Key)
	resourceConfig := m2.SlotResourceConfig{
		ResourceConfig: pb.ResourceConfig{
			MemoryInMegabytes: s.MetaData.MemoryInMb,
		},
	}
	start := time.Now()
	slot, err := s.platformClient.CreateSlot(ctx, requestId, &resourceConfig)
	if err != nil {
		errorMessage := fmt.Sprintf("create slot failed with: %s", err.Error())
		m2.Printf("createFailed:%s", errorMessage)
		return
	}
	if s.OnInit < 1 && s.idleInstance.Len() > 0 {
		m2.Printf("fastDelete:%s", meta.Key)
		m2.Printf("fastConsume1,%s,%d", s.MetaData.Key, time.Since(start).Milliseconds())
		s.deleteSlot(ctx, requestId, slot.Id, instanceId, meta.Key, "fastDelete")
		return
	}
	m2.Printf("AssignInit,%s,%s", requestId, meta.Key)
	initRec := make(chan *m2.Instance)
	rMeta := &m2.Meta{
		Meta: pb.Meta{
			Key:           meta.Key,
			Runtime:       meta.Runtime,
			TimeoutInSecs: meta.TimeoutInSecs,
		},
	}
	go s.init(initRec, requestId, instanceId, slot, rMeta, start)
	going := true
	for going {
		select {
		case instance := <-initRec:
			if instance != nil {
				result.Instance = instance
				select {
				case s.IdleChan <- result:
					s.mu.Lock()
					s.instances[instance.Id] = instance
					s.mu.Unlock()
					m2.Printf("pushIdleSuc:%s", meta.Key)
					m2.Printf("initConsume,%s,%d", s.MetaData.Key, time.Since(start).Milliseconds())
				case <-time.After(100 * time.Millisecond):
					s.deleteSlot(ctx, requestId, slot.Id, instanceId, meta.Key, "fastDelete")
					m2.Printf("fastConsume2,%s,%d", s.MetaData.Key, time.Since(start).Milliseconds())
				}
			}
			going = false
			break
		case <-time.Tick(100 * time.Millisecond):
			if s.OnInit < 1 && s.idleInstance.Len() > 0 {
				m2.Printf("fastConsume4,%s,%d", s.MetaData.Key, time.Since(start).Milliseconds())
				s.deleteSlot(ctx, requestId, slot.Id, instanceId, meta.Key, "fastDelete4")
				going = false
				break
			}
		}
	}
}

func (s *Simple) init(initRec chan *m2.Instance, requestId string, instId string, slot *m2.Slot, rMeta *m2.Meta, start time.Time) {
	instance, err := s.platformClient.Init(context.Background(), requestId, instId, slot, rMeta)
	if err != nil {
		errorMessage := fmt.Sprintf("create instance failed with: %s", err.Error())
		m2.Printf(errorMessage)
		return
	}
	select {
	case initRec <- instance:
		m2.Printf("initSendSuccess")
	default:
		s.deleteSlot(context.Background(), requestId, slot.Id, instId, rMeta.Key, "fastDelete3")
		m2.Printf("fastConsume3,%s,%d", s.MetaData.Key, time.Since(start).Milliseconds())
	}
}
