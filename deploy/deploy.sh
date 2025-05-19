# deploy/deploy.sh
#!/usr/bin/env bash
set -eux

REGISTRY="registry.local:5000"
NS="wallet"

# 0. 环境准备
minikube start --driver=docker --insecure-registry="${REGISTRY}"
minikube addons enable ingress
docker run -d --restart=always -p 5000:5000 --name registry-local registry:2 || true
grep -q "registry.local" /etc/hosts || echo "127.0.0.1 registry.local" | sudo tee -a /etc/hosts
IP=$(minikube ip); grep -q "wallet.local" /etc/hosts || echo "$IP wallet.local" | sudo tee -a /etc/hosts

# 1. 镜像
docker build -t ${REGISTRY}/wallet-server:latest -f cmd/server/Dockerfile .
docker push  ${REGISTRY}/wallet-server:latest
docker build -t ${REGISTRY}/wallet-poller:latest -f cmd/poller/Dockerfile .
docker push  ${REGISTRY}/wallet-poller:latest

# 2. K8s 资源
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/wallet-db-secret.yaml
kubectl apply -f deploy/k8s/wallet-config.yaml
kubectl create configmap wallet-init-sql --from-file=schema.sql -n ${NS} --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/k8s/redis/
kubectl apply -f deploy/k8s/postgres/
kubectl apply -f deploy/k8s/zookeeper/
kubectl apply -f deploy/k8s/kafka/
kubectl apply -f deploy/k8s/wallet/
kubectl apply -f deploy/k8s/ingress.yaml

# 3. 等待就绪
for D in redis postgres zookeeper kafka wallet-server wallet-poller; do
  kubectl -n ${NS} rollout status deploy/${D} --timeout=180s
done

echo "✅ all done -> http://wallet.local"