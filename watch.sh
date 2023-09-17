export VERSION=v841
export DATA=ds1a
export NAME=sim9
docker buildx build --push --platform linux/amd64 \
    --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64  \
    -f Dockerfile . -t registry.cn-hangzhou.aliyuncs.com/beeper/scaler:$VERSION
envsubst < sim.yaml > sim2.yaml
kubectl apply -f sim2.yaml
sleep 15
kubectl logs -f jobs/$NAME scaler > ~/Documents/data_training/$VERSION.log