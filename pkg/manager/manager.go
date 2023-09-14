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
	"github.com/AliyunContainerService/scaler/go/pkg/config"
	"github.com/AliyunContainerService/scaler/go/pkg/model"
	scaler2 "github.com/AliyunContainerService/scaler/go/pkg/scaler"
	"sync"
)

type Manager struct {
	rw           sync.RWMutex
	schedulers   map[string]*scaler2.BaseScheduler
	resourceChan map[uint64]chan *model.Slot
	config       *config.Config
	d            *scaler2.Dispatcher
}
