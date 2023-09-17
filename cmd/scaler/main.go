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

package main

import (
	"github.com/AliyunContainerService/scaler/go/pkg/server"
	pb "github.com/AliyunContainerService/scaler/proto"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
)

func main() {
	run()
}
func run() {
	debug.SetGCPercent(150)
	zerolog.TimeFieldFormat = "15:04:05.000000"
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(
		grpc.MaxConcurrentStreams(1000),
		grpc.InitialWindowSize(1024*1024),
		grpc.InitialConnWindowSize(16*1024*1024),
		//grpc.ReadBufferSize(1024*512),
		//grpc.WriteBufferSize(1024*512)
	)
	scaleServer := server.New()
	pb.RegisterScalerServer(s, scaleServer)
	log.Printf("server listening at %v", lis.Addr())

	if os.Getenv("dev") != "true" {
		runtime.SetCPUProfileRate(0)
		runtime.SetBlockProfileRate(0)
		runtime.SetMutexProfileFraction(0)
	}
	go func() {
		if os.Getenv("dev") == "true" {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}
	}()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
