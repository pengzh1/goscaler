export VERSION=v851
export DATA=ds1a
export NAME=sim1
export DATA2=ds2a
export NAME2=sim2
#docker buildx build --push --platform linux/amd64 \
#    --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64  \
#    -f Dockerfile . -t registry.cn-hangzhou.aliyuncs.com/beeper/scaler:$VERSION
envsubst < sim.yaml > sim2.yaml
kubectl apply -f sim2.yaml
sleep 15
kubectl logs -f jobs/$NAME scaler > ~/Documents/data_training/$VERSION.log1 &
kubectl logs -f jobs/$NAME2 scaler > ~/Documents/data_training/$VERSION.log2 &