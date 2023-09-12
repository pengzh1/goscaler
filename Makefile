.PHONY: binary, proto

default: binary


# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

clean:
	-rm -Rf _output

binary: proto
	go build -o scaler cmd/scaler/main.go

docker-build: proto
#	manifest/make-rules/manifest.sh ${IMAGE_REPO}/edge-proxy:${GIT_VERSION} ${DOCKER_USERNAME} ${DOCKER_PASSWD}
	docker buildx build --push --platform linux/amd64 \
    --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64  \
    -f Dockerfile . -t registry.cn-hangzhou.aliyuncs.com/beeper/scaler:v7.0.2

proto:
	cd proto && protoc -I ./  serverless-sim.proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative && cd -

