# E2E 测试本地运行指南

## 前置条件

1. 有可用的 Kubernetes 集群（通过 kubeconfig 访问）
2. 已安装 cert-manager
3. 已安装 LWS (LeaderWorkerSet) CRD
4. 已安装 RBG (RoleBasedGroupSet) CRD

## 启动依赖服务

### 1. 启动 RBG Controller（在 RBG 项目目录）

```bash
cd /path/to/rbg
go run ./cmd/rbgs/main.go --metrics-bind-address=:8080 --health-probe-bind-address=:8081
```

### 2. 启动 Arks Controller（在 Arks 项目目录）

```bash
go run ./cmd/main.go --metrics-bind-address=:8082 --health-probe-bind-address=:8083
```

## 运行 E2E 测试

### 运行所有测试（排除真实 GPU 和 Manager 测试）

```bash
USE_EXISTING_CLUSTER=true go test -v -count=1 -timeout 30m ./test/e2e \
  -args -ginkgo.v -ginkgo.label-filter='!real-gpu' -ginkgo.skip='Manager'
```

### 运行特定标签的测试

```bash
# 基础 CRUD 测试
USE_EXISTING_CLUSTER=true go test -v -count=1 -timeout 30m ./test/e2e \
  -args -ginkgo.vv -ginkgo.label-filter='basic'

# Coordination 相关测试
USE_EXISTING_CLUSTER=true go test -v -count=1 -timeout 30m ./test/e2e \
  -args -ginkgo.vv -ginkgo.label-filter='coordination'

# LWS partition 验证测试
USE_EXISTING_CLUSTER=true go test -v -count=1 -timeout 30m ./test/e2e \
  -args -ginkgo.vv -ginkgo.label-filter='lws-partition-verify'

# Gang 调度测试
USE_EXISTING_CLUSTER=true go test -v -count=1 -timeout 30m ./test/e2e \
  -args -ginkgo.vv -ginkgo.label-filter='lws-gang'

# GPU 资源测试（模拟）
USE_EXISTING_CLUSTER=true go test -v -count=1 -timeout 30m ./test/e2e \
  -args -ginkgo.vv -ginkgo.label-filter='gpu-resources && !real-gpu'
```

### 运行特定名称的测试

```bash
USE_EXISTING_CLUSTER=true go test -v -count=1 -timeout 30m ./test/e2e \
  -args -ginkgo.vv -ginkgo.focus='should set LWS partition'
```

## 常见问题

### CRD Terminating 卡住

```bash
# 清理 finalizers
kubectl patch crd arksmodels.arks.ai -p '{"metadata":{"finalizers":[]}}' --type=merge

# 重新安装 CRD
make install
```

### 测试 namespace 残留

```bash
kubectl get ns | grep e2e | awk '{print $1}' | xargs -r kubectl delete ns --force --grace-period=0
```

## 可用的测试标签

| 标签 | 说明 |
|------|------|
| `basic` | 基础 CRUD 操作 |
| `scaling` | 扩缩容测试 |
| `rolling` | 滚动更新测试 |
| `coordination` | Coordination 功能测试 |
| `lws-gang` | LWS Gang 调度测试 |
| `lws-partition-verify` | LWS partition 验证 |
| `gpu-resources` | GPU 资源测试 |
| `real-gpu` | 真实 GPU 测试（需要 GPU 节点）|
| `recovery` | 故障恢复测试 |
