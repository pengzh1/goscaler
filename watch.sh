export VERSION=v913
export DATA=0307
export NAME=sim1
export DATA2=8b83S
export NAME2=sim2
#export DATA3=crazy3
#export NAME3=sim3
docker buildx build --push --platform linux/amd64 \
    --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64  \
    -f Dockerfile . -t registry.cn-hangzhou.aliyuncs.com/beeper/scaler:$VERSION
envsubst < sim.yaml > sim2.yaml
kubectl apply -f sim2.yaml
sleep 10
kubectl logs -f jobs/$NAME scaler > ~/Documents/data_training/$VERSION.log1 &
kubectl logs -f jobs/$NAME2 scaler > ~/Documents/data_training/$VERSION.log2 &
#kubectl logs -f jobs/$NAME3 scaler > ~/Documents/data_training/$VERSION.log3 &