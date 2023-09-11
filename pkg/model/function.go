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

package model

import (
	"log"
	"time"

	pb "github.com/AliyunContainerService/scaler/proto"
)

type Meta struct {
	pb.Meta
}

type Instance struct {
	Id               string
	Slot             *Slot
	Meta             *Meta
	CreateTimeInMs   int64
	InitDurationInMs int64
	Busy             bool
	//CreateTime       time.Time
	LastIdleTime time.Time
	LastStart    time.Time
	UseTime      *[7200]uint16
}

func (inst *Instance) UpdateActive(start time.Time, end time.Time, sUse *[7200]uint16) {
	startMill := start.Sub(CreateTime).Milliseconds()
	endMill := end.Sub(CreateTime).Milliseconds()
	if startMill/1000 != endMill/1000 {
		log.Printf("updateActive %d %d %d", startMill, endMill, endMill-startMill)
		for i := startMill / 1000; i <= endMill/1000; i++ {
			use := 1000
			if i == startMill/1000 {
				use = int((i+1)*1000 - startMill)
			} else if i == endMill/1000 {
				use = int(endMill - i*1000)
			}
			inst.UseTime[i] += uint16(use)
			sUse[i] += uint16(use)
		}
	} else {
		log.Printf("updateActive %d %d", startMill/1000, endMill-startMill)
		inst.UseTime[startMill/1000] += uint16(endMill - startMill)
		sUse[startMill/1000] += uint16(endMill - startMill)
	}
}

func (inst *Instance) CountUsage(secs int) float32 {
	curSecs := int(time.Now().Sub(CreateTime).Milliseconds() / 1000)
	if curSecs < secs/2 {
		return 1
	}
	startSecs := curSecs - secs
	if startSecs <= 0 {
		startSecs = 0
	}
	useMills := 0
	haveMills := false
	secCnt := 0
	for i := startSecs; i <= curSecs; i++ {
		if inst.UseTime[i] > 0 {
			useMills += int(inst.UseTime[i])
			haveMills = true
			if haveMills {
				secCnt += 1
			}

		}
	}
	if inst.Busy {
		useMills += int(time.Now().Sub(inst.LastStart).Milliseconds())
	}
	log.Printf("usageCount %d,%d,%d,%t,%d", curSecs, startSecs, useMills, haveMills, secCnt)
	return float32(useMills) / float32(secs*1000)
}
