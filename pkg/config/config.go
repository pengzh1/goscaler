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

package config

import "time"

type Config struct {
	ClientAddr           string
	GcInterval           time.Duration
	IdleDurationBeforeGC time.Duration
}

var DefaultConfig = Config{
	ClientAddr:           "127.0.0.1:50051",
	GcInterval:           10 * time.Second,
	IdleDurationBeforeGC: 40 * time.Second,
}

func (c *Config) GetGcInterval(initDuration float32, avgExec float32) time.Duration {
	if avgExec > 1000 && avgExec/initDuration >= 1 {
		return time.Second
	}
	return c.GcInterval
}

func (c *Config) GetIdleDurationBeforeGC(initDuration float32, avgExec float32) time.Duration {
	if avgExec > 1000 && avgExec/initDuration >= 1 {
		return time.Second
	}
	return c.IdleDurationBeforeGC
}

func (c *Config) GetUsageCheckInterval(avgExec float32) time.Duration {
	if avgExec > 0 && avgExec < 100 {
		return 2 * time.Second
	} else if avgExec > 100 {
		return time.Duration(int(avgExec*10/1000)+1) * time.Second
	}
	return 30 * time.Second
}

func (c *Config) GetUsageLimit() float32 {
	return 0.2
}
