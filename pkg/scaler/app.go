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
	"github.com/AliyunContainerService/scaler/go/pkg/kmeans"
	m2 "github.com/AliyunContainerService/scaler/go/pkg/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/AliyunContainerService/scaler/proto"
	"github.com/google/uuid"
)

type BaseScheduler struct {
	config         *config.Config
	MetaData       *m2.Meta
	Lock           sync.Mutex
	CheckLock      sync.Mutex
	IdleLock       sync.Mutex
	instances      sync.Map
	idleInstance   *list.List
	OnWait         atomic.Int32
	OnCreate       atomic.Int32
	IdleSize       atomic.Int32
	InstanceSize   atomic.Int32
	IdleChan       chan *m2.CreateRet
	CheckChan      sync.Map // key timestamp int64 value chan struct{}
	LastGc         time.Time
	LastKm         time.Time
	CreateTime     time.Time
	GcTime         uint64
	GcCnt          int
	ReqCnt         int
	EndCnt         int
	ITLock         sync.Mutex
	LastTime       int64
	ITTime         *[200]int64
	ITData         *[200]int32
	LastExec       *[20]int32
	MaxIT          int32
	InitMs         int32
	AvgExec        int32
	Rule           *Rule
	d              *Dispatcher
	LastMaxWorking int32
	CurMaxWorking  int32
	CurWorking     atomic.Int32
	pool           *SlotPool
}

func NewBaseScheduler(metaData *m2.Meta, config *config.Config, d *Dispatcher) *BaseScheduler {
	scheduler := &BaseScheduler{
		config:       config,
		MetaData:     metaData,
		instances:    sync.Map{},
		idleInstance: list.New(),
		OnWait:       atomic.Int32{},
		OnCreate:     atomic.Int32{},
		IdleSize:     atomic.Int32{},
		InstanceSize: atomic.Int32{},
		IdleChan:     make(chan *m2.CreateRet, 1024),
		CheckChan:    sync.Map{},
		LastGc:       time.Now(),
		LastKm:       time.Now(),
		CreateTime:   time.Now(),
		ITData:       new([200]int32),
		ITTime:       new([200]int64),
		LastExec:     new([20]int32),
		Rule: &Rule{
			Cate: "",
		},
		d: d,
	}
	m2.Printf("NewBaseScheduler scaler for app: %s is created", metaData.Key)
	return scheduler
}

func (s *BaseScheduler) Assign(ctx context.Context, request *pb.AssignRequest) (*pb.AssignReply, error) {
	cur := s.CurWorking.Add(1)
	if cur > s.CurMaxWorking {
		s.CurMaxWorking = cur
		if s.CurMaxWorking > s.LastMaxWorking {
			s.LastMaxWorking = s.CurMaxWorking
		}
	}
	start := time.Now()
	s.ITLock.Lock()
	if s.LastTime == 0 {
		s.LastTime = start.UnixMilli()
	} else {
		it := int32(start.UnixMilli() - s.LastTime)
		if it > s.MaxIT {
			s.MaxIT = it
		}
		s.ITTime[(s.ReqCnt-1)%len(s.ITTime)] = start.UnixMilli()
		s.ITData[(s.ReqCnt-1)%len(s.ITTime)] = it
		if s.Rule.Valid {
			// IT逃逸后设置规则无效
			if kmeans.Escape(it, s.Rule.Cluster[0]) && kmeans.Escape(it, s.Rule.Cluster[1]) {
				s.Rule.Valid = false
				m2.Printf("ruleInvalid:%s,%d", s.MetaData.Key, it)
			}
		}
		s.LastTime = start.UnixMilli()
	}
	s.ReqCnt += 1
	s.ITLock.Unlock()
	s.pool.ReqRecord(s)
	instanceId := uuid.New().String()
	if m2.Dev {
		defer func() {
			m2.ReqLog(request.RequestId, s.MetaData.Key, instanceId, "AssignEnd", time.Since(start).Milliseconds())
		}()
		m2.ReqLog(request.RequestId, s.MetaData.Key, "", "AssignStart", -1)
	}
	instance := s.d.Fetch(instanceId, request.RequestId, s.MetaData)
	s.InitMs = int32(instance.InitDurationInMs)
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

func (s *BaseScheduler) Idle(ctx context.Context, request *pb.IdleRequest) (*pb.IdleReply, error) {
	if request.Assigment == nil {
		return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("assignment is nil"))
	}
	cur := s.CurWorking.Add(-1)
	if cur == 0 {
		s.LastMaxWorking = s.CurMaxWorking
	}
	start := time.Now()
	instanceId := request.Assigment.InstanceId
	if m2.Dev {
		defer func() {
			m2.ReqLog(request.Assigment.RequestId, s.MetaData.Key, "", "IdleEnd", time.Since(start).Microseconds())
		}()
		m2.ReqLog(request.Assigment.RequestId, s.MetaData.Key, "", "IdleStart"+request.GetResult().String(), time.Since(start).Microseconds())
	}
	needDestroy := false
	if request.Result != nil && request.Result.NeedDestroy != nil && *request.Result.NeedDestroy {
		needDestroy = true
	}
	s.ITLock.Lock()
	s.EndCnt += 1
	s.LastExec[(s.EndCnt-1)%len(s.LastExec)] = int32(request.Result.DurationInMs)
	if s.EndCnt%5 == 0 {
		sum, cnt := int32(0), 0
		for _, c := range s.LastExec {
			if c > 0 {
				sum += c
				cnt += 1
			}
		}
		s.AvgExec = sum / int32(cnt)
	}
	s.ITLock.Unlock()
	s.pool.EndRecord(int32(request.Result.DurationInMs))
	_ = s.d.Idle(instanceId, uuid.NewString(), s.MetaData.Key, "idle", needDestroy)
	return &pb.IdleReply{
		Status:       pb.Status_Ok,
		ErrorMessage: nil,
	}, nil
}
