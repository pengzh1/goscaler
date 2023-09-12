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

package manager

import (
	"fmt"
	"github.com/AliyunContainerService/scaler/go/pkg/config"
	"github.com/AliyunContainerService/scaler/go/pkg/model"
	scaler2 "github.com/AliyunContainerService/scaler/go/pkg/scaler"
	"log"
	"sync"
	"time"
)

type Manager struct {
	rw           sync.RWMutex
	schedulers   map[string]*scaler2.Simple
	resourceChan map[uint64]chan *model.Slot
	config       *config.Config
}

func New(config *config.Config) *Manager {
	return &Manager{
		rw:           sync.RWMutex{},
		schedulers:   make(map[string]*scaler2.Simple),
		resourceChan: make(map[uint64]chan *model.Slot),
		config:       config,
	}
}

func (m *Manager) GetOrCreate(metaData *model.Meta) scaler2.Scaler {
	m.rw.RLock()
	if scheduler := m.schedulers[metaData.Key]; scheduler != nil {
		m.rw.RUnlock()
		return scheduler
	}
	m.rw.RUnlock()

	m.rw.Lock()
	if scheduler := m.schedulers[metaData.Key]; scheduler != nil {
		m.rw.Unlock()
		return scheduler
	}
	log.Printf("Create new scaler for app %s", metaData.Key)
	if _, ok := m.resourceChan[metaData.MemoryInMb]; !ok {
		m.resourceChan[metaData.MemoryInMb] = make(chan *model.Slot)
	}
	scheduler := scaler2.New(metaData, m.config, m.resourceChan[metaData.MemoryInMb])
	m.schedulers[metaData.Key] = scheduler
	m.rw.Unlock()
	return scheduler
}

func (m *Manager) Get(metaKey string) (scaler2.Scaler, error) {
	m.rw.RLock()
	defer m.rw.RUnlock()
	if scheduler := m.schedulers[metaKey]; scheduler != nil {
		return scheduler, nil
	}
	return nil, fmt.Errorf("scaler of app: %s not found", metaKey)
}

func (m *Manager) GcLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.rw.RLock()
			for _, v := range m.schedulers {
				v.GC()
			}
			m.rw.RUnlock()
		}
	}
}
