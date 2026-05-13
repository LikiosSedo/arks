# Application Usage

## Overview

`ArksApplication` is a Kubernetes Custom Resource Definition (CRD) specifically designed for serving large language models (LLMs). It provides a simplified solution for users to deploy production-grade LLM services with a single command. Through high-level abstraction, it hides the complexity of the underlying infrastructure, allowing developers to focus on business logic rather than operational details. The overall architecture of ArksApplication is illustrated in the following diagram:

![application-architecture-v1](images/application-architecture-v1.png)

The core capabilities of `ArksApplication` include:
- Unified Multi-Runtime Interface: Seamlessly integrates with various high-performance inference runtimes without exposing runtime-specific complexities.
  - vLLM
  - SGLang
  - Dynamo
- Two deployment topologies via `spec.mode`:
  - `unified` (default): single inference role that runs the full pipeline; supports single-node and multi-node (LWS group) inference; optional router for multi-replica load balancing.
  - `disaggregated`: prefill / decode roles separated, fronted by a router. Currently only supports `runtime: sglang`.
- Multi-model deployment support
- Heterogeneous deployment support
- Unified metrics interface

For the complete field-by-field reference, including mode/role validation
rules and command override semantics, see
[arksapplication-unified-guide.md](./arksapplication-unified-guide.md).

## Usage Guide

*Prerequisites*
- Ensure an `ArksModel` is configured for preloading the model.

### Deploying Multi-Node Inference Service (vLLM)

```yaml
apiVersion: arks.ai/v1
kind: ArksApplication
metadata:
  labels:
    app.kubernetes.io/name: arks-operator
    app.kubernetes.io/managed-by: kustomize
  name: app-qwen-vllm
spec:
  mode: unified
  runtime: vllm
  model:
    name: qwen-7b
  # Optional, default to spec.model.name
  # servedModelName: qwen-7b
  unified:
    replicas: 1
    size: 2                       # 2-node LWS group (leader + worker)
    runtimeCommonArgs:
      - --tensor-parallel-size
      - "2"
    instanceSpec:
      resources:
        limits:
          nvidia.com/gpu: "1"
        requests:
          nvidia.com/gpu: "1"
```

Spec field meanings (selected):
- `spec.mode`: `unified | disaggregated`. Defaults to `unified`.
- `spec.runtime`: `vllm | sglang | dynamo`. Defaults to `vllm`.
- `spec.runtimeImage` (optional): Override the controller's default runtime image.
- `spec.runtimeImagePullSecrets` (optional): Image pull secrets for private registries.
- `spec.model.name`: ArksModel resource name.
- `spec.servedModelName` (optional): Model name exposed on the OpenAI-compatible API. Defaults to `spec.model.name`.
- `spec.unified.replicas`: Number of inference instances. Each instance is a single LWS group.
- `spec.unified.size`: Number of pods per group (1 = single node, >1 = multi-node distributed inference).
- `spec.unified.runtimeCommonArgs`: Additional arguments passed to the underlying runtime binary.
- `spec.unified.instanceSpec`: Pod template (resources, volumes, env, scheduling, etc.).

View the status:

```
$ kubectl get arksapplications app-qwen-vllm
NAME            PHASE     MODE      AGE   REPLICAS   READY
app-qwen-vllm   Running   unified   17h   1          1
$ kubectl get pods -l arks.ai/application=app-qwen-vllm
NAME                                READY   STATUS    RESTARTS   AGE
app-qwen-vllm-0-unified-0           1/1     Running   0          17h
app-qwen-vllm-0-unified-0-1         1/1     Running   0          17h
```

### Deploying Multi-Node Inference Service (SGLang)

```yaml
apiVersion: arks.ai/v1
kind: ArksApplication
metadata:
  labels:
    app.kubernetes.io/name: arks-operator
    app.kubernetes.io/managed-by: kustomize
  name: app-qwen
spec:
  mode: unified
  runtime: sglang
  model:
    name: qwen-7b
  unified:
    replicas: 1
    size: 2
    runtimeCommonArgs:
      - --tp
      - "2"
      - --mem-fraction-static
      - "0.7"
    instanceSpec:
      resources:
        limits:
          nvidia.com/gpu: "1"
        requests:
          nvidia.com/gpu: "1"
```

### Deploying Disaggregated (Prefill / Decode) Inference

Only `runtime: sglang` is currently supported in disaggregated mode. See
the [unified-guide](./arksapplication-unified-guide.md#mode-4--disaggregated-prefill--decode--router)
for the full template.

### SLO-based HPA

`ArksApplication` HPA features are under development.

## Migration from Legacy CRDs

The legacy single-node `ArksApplication` schema (top-level `replicas`/`size`/`tensorParallelSize`/`instanceSpec`) and the legacy `ArksDisaggregatedApplication` CRD are both superseded by the unified `ArksApplication` schema above. See [arksapplication-unified-guide.md](./arksapplication-unified-guide.md) for the field mapping table. The legacy `ArksDisaggregatedApplication` CRD remains registered and continues to serve existing CRs.
