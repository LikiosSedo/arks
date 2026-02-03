/*
Copyright 2025.

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

package wrappers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

const (
	// DefaultMockImage is the default mock image for testing (using siflow registry)
	// Using doks-debug which stays running (unlike busybox which exits immediately)
	DefaultMockImage = "registry-cn-shanghai.siflow.cn/k8s/digitalocean/doks-debug:latest"
	// DefaultRouterImage is the default mock router image
	DefaultRouterImage = "registry-cn-shanghai.siflow.cn/k8s/digitalocean/doks-debug:latest"
	// DefaultRuntimeImage is the default mock runtime image
	DefaultRuntimeImage = "registry-cn-shanghai.siflow.cn/k8s/digitalocean/doks-debug:latest"
)

// ArksDisaggAppWrapper wraps ArksDisaggregatedApplication for builder pattern
type ArksDisaggAppWrapper struct {
	arksv1.ArksDisaggregatedApplication
}

// Obj returns the underlying ArksDisaggregatedApplication object
func (w *ArksDisaggAppWrapper) Obj() *arksv1.ArksDisaggregatedApplication {
	return &w.ArksDisaggregatedApplication
}

// WithName sets the name
func (w *ArksDisaggAppWrapper) WithName(name string) *ArksDisaggAppWrapper {
	w.Name = name
	return w
}

// WithNamespace sets the namespace
func (w *ArksDisaggAppWrapper) WithNamespace(ns string) *ArksDisaggAppWrapper {
	w.Namespace = ns
	return w
}

// WithModel sets the model reference
func (w *ArksDisaggAppWrapper) WithModel(modelName string) *ArksDisaggAppWrapper {
	w.Spec.Model = corev1.LocalObjectReference{Name: modelName}
	return w
}

// WithRuntime sets the runtime type
func (w *ArksDisaggAppWrapper) WithRuntime(runtime string) *ArksDisaggAppWrapper {
	w.Spec.Runtime = runtime
	return w
}

// WithRouterImage sets the router image
func (w *ArksDisaggAppWrapper) WithRouterImage(image string) *ArksDisaggAppWrapper {
	w.Spec.RouterImage = image
	return w
}

// WithRuntimeImage sets the runtime image
func (w *ArksDisaggAppWrapper) WithRuntimeImage(image string) *ArksDisaggAppWrapper {
	w.Spec.RuntimeImage = image
	return w
}

// WithRouterReplicas sets the router replicas
func (w *ArksDisaggAppWrapper) WithRouterReplicas(replicas int32) *ArksDisaggAppWrapper {
	w.Spec.Router.Replicas = ptr.To(replicas)
	return w
}

// WithPrefillReplicas sets the prefill replicas
func (w *ArksDisaggAppWrapper) WithPrefillReplicas(replicas int32) *ArksDisaggAppWrapper {
	w.Spec.Prefill.Replicas = ptr.To(replicas)
	return w
}

// WithDecodeReplicas sets the decode replicas
func (w *ArksDisaggAppWrapper) WithDecodeReplicas(replicas int32) *ArksDisaggAppWrapper {
	w.Spec.Decode.Replicas = ptr.To(replicas)
	return w
}

// WithPrefillSize sets the prefill size (workers per group)
func (w *ArksDisaggAppWrapper) WithPrefillSize(size int) *ArksDisaggAppWrapper {
	w.Spec.Prefill.Size = size
	return w
}

// WithDecodeSize sets the decode size (workers per group)
func (w *ArksDisaggAppWrapper) WithDecodeSize(size int) *ArksDisaggAppWrapper {
	w.Spec.Decode.Size = size
	return w
}

// WithRouterResources sets the router resource requirements
func (w *ArksDisaggAppWrapper) WithRouterResources(resources corev1.ResourceRequirements) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.Resources = resources
	return w
}

// WithPrefillResources sets the prefill resource requirements
func (w *ArksDisaggAppWrapper) WithPrefillResources(resources corev1.ResourceRequirements) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.Resources = resources
	return w
}

// WithDecodeResources sets the decode resource requirements
func (w *ArksDisaggAppWrapper) WithDecodeResources(resources corev1.ResourceRequirements) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.Resources = resources
	return w
}

// WithRouterEnv sets the router environment variables
func (w *ArksDisaggAppWrapper) WithRouterEnv(envs []corev1.EnvVar) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.Env = envs
	return w
}

// WithPrefillEnv sets the prefill environment variables
func (w *ArksDisaggAppWrapper) WithPrefillEnv(envs []corev1.EnvVar) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.Env = envs
	return w
}

// WithDecodeEnv sets the decode environment variables
func (w *ArksDisaggAppWrapper) WithDecodeEnv(envs []corev1.EnvVar) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.Env = envs
	return w
}

// WithRouterReadinessProbe sets the router readiness probe
func (w *ArksDisaggAppWrapper) WithRouterReadinessProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.ReadinessProbe = probe
	return w
}

// WithPrefillReadinessProbe sets the prefill readiness probe
func (w *ArksDisaggAppWrapper) WithPrefillReadinessProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.ReadinessProbe = probe
	return w
}

// WithDecodeReadinessProbe sets the decode readiness probe
func (w *ArksDisaggAppWrapper) WithDecodeReadinessProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.ReadinessProbe = probe
	return w
}

// WithRouterLivenessProbe sets the router liveness probe
func (w *ArksDisaggAppWrapper) WithRouterLivenessProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.LivenessProbe = probe
	return w
}

// WithPrefillLivenessProbe sets the prefill liveness probe
func (w *ArksDisaggAppWrapper) WithPrefillLivenessProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.LivenessProbe = probe
	return w
}

// WithDecodeLivenessProbe sets the decode liveness probe
func (w *ArksDisaggAppWrapper) WithDecodeLivenessProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.LivenessProbe = probe
	return w
}

// WithRouterVolumes sets the router volumes
func (w *ArksDisaggAppWrapper) WithRouterVolumes(volumes []corev1.Volume) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.Volumes = volumes
	return w
}

// WithPrefillVolumes sets the prefill volumes
func (w *ArksDisaggAppWrapper) WithPrefillVolumes(volumes []corev1.Volume) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.Volumes = volumes
	return w
}

// WithDecodeVolumes sets the decode volumes
func (w *ArksDisaggAppWrapper) WithDecodeVolumes(volumes []corev1.Volume) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.Volumes = volumes
	return w
}

// WithRouterVolumeMounts sets the router volume mounts
func (w *ArksDisaggAppWrapper) WithRouterVolumeMounts(mounts []corev1.VolumeMount) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.VolumeMounts = mounts
	return w
}

// WithPrefillVolumeMounts sets the prefill volume mounts
func (w *ArksDisaggAppWrapper) WithPrefillVolumeMounts(mounts []corev1.VolumeMount) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.VolumeMounts = mounts
	return w
}

// WithDecodeVolumeMounts sets the decode volume mounts
func (w *ArksDisaggAppWrapper) WithDecodeVolumeMounts(mounts []corev1.VolumeMount) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.VolumeMounts = mounts
	return w
}

// WithRouterArgs sets the router arguments
func (w *ArksDisaggAppWrapper) WithRouterArgs(args []string) *ArksDisaggAppWrapper {
	w.Spec.Router.RouterArgs = args
	return w
}

// WithPrefillRuntimeArgs sets the prefill runtime arguments
func (w *ArksDisaggAppWrapper) WithPrefillRuntimeArgs(args []string) *ArksDisaggAppWrapper {
	w.Spec.Prefill.RuntimeCommonArgs = args
	return w
}

// WithDecodeRuntimeArgs sets the decode runtime arguments
func (w *ArksDisaggAppWrapper) WithDecodeRuntimeArgs(args []string) *ArksDisaggAppWrapper {
	w.Spec.Decode.RuntimeCommonArgs = args
	return w
}

// WithRouterCommandOverride sets the router command override
func (w *ArksDisaggAppWrapper) WithRouterCommandOverride(command []string) *ArksDisaggAppWrapper {
	w.Spec.Router.CommandOverride = command
	return w
}

// WithPrefillLeaderCommandOverride sets the prefill leader command override
func (w *ArksDisaggAppWrapper) WithPrefillLeaderCommandOverride(command []string) *ArksDisaggAppWrapper {
	w.Spec.Prefill.LeaderCommandOverride = command
	return w
}

// WithPrefillWorkerCommandOverride sets the prefill worker command override
func (w *ArksDisaggAppWrapper) WithPrefillWorkerCommandOverride(command []string) *ArksDisaggAppWrapper {
	w.Spec.Prefill.WorkerCommandOverride = command
	return w
}

// WithDecodeLeaderCommandOverride sets the decode leader command override
func (w *ArksDisaggAppWrapper) WithDecodeLeaderCommandOverride(command []string) *ArksDisaggAppWrapper {
	w.Spec.Decode.LeaderCommandOverride = command
	return w
}

// WithDecodeWorkerCommandOverride sets the decode worker command override
func (w *ArksDisaggAppWrapper) WithDecodeWorkerCommandOverride(command []string) *ArksDisaggAppWrapper {
	w.Spec.Decode.WorkerCommandOverride = command
	return w
}

// WithMockCommands sets command overrides for all roles to use sleep (for mock testing)
func (w *ArksDisaggAppWrapper) WithMockCommands() *ArksDisaggAppWrapper {
	mockCmd := []string{"sleep", "infinity"}
	w.Spec.Router.CommandOverride = mockCmd
	w.Spec.Prefill.LeaderCommandOverride = mockCmd
	w.Spec.Prefill.WorkerCommandOverride = mockCmd
	w.Spec.Decode.LeaderCommandOverride = mockCmd
	w.Spec.Decode.WorkerCommandOverride = mockCmd
	return w
}

// WithPodGroupPolicy sets the PodGroup policy for gang scheduling
func (w *ArksDisaggAppWrapper) WithPodGroupPolicy(policy *arksv1.PodGroupPolicy) *ArksDisaggAppWrapper {
	w.Spec.PodGroupPolicy = policy
	return w
}

// WithCoordinationPolicy sets the CoordinationPolicy for coordinated scaling
// This is independent of PodGroupPolicy and can be used with LWS-level gang scheduling
func (w *ArksDisaggAppWrapper) WithCoordinationPolicy(policy *arksv1.CoordinationPolicy) *ArksDisaggAppWrapper {
	w.Spec.CoordinationPolicy = policy
	return w
}

// WithPrefillSchedulerName sets the prefill scheduler name (for LWS-level gang scheduling)
func (w *ArksDisaggAppWrapper) WithPrefillSchedulerName(schedulerName string) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.SchedulerName = schedulerName
	return w
}

// WithDecodeSchedulerName sets the decode scheduler name (for LWS-level gang scheduling)
func (w *ArksDisaggAppWrapper) WithDecodeSchedulerName(schedulerName string) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.SchedulerName = schedulerName
	return w
}

// WithLabels sets custom labels
func (w *ArksDisaggAppWrapper) WithLabels(labels map[string]string) *ArksDisaggAppWrapper {
	if w.Labels == nil {
		w.Labels = make(map[string]string)
	}
	for k, v := range labels {
		w.Labels[k] = v
	}
	return w
}

// WithAnnotations sets custom annotations
func (w *ArksDisaggAppWrapper) WithAnnotations(annotations map[string]string) *ArksDisaggAppWrapper {
	if w.Annotations == nil {
		w.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		w.Annotations[k] = v
	}
	return w
}

// WithPrefillNodeSelector sets the prefill node selector
func (w *ArksDisaggAppWrapper) WithPrefillNodeSelector(selector map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.NodeSelector = selector
	return w
}

// WithDecodeNodeSelector sets the decode node selector
func (w *ArksDisaggAppWrapper) WithDecodeNodeSelector(selector map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.NodeSelector = selector
	return w
}

// WithRouterNodeSelector sets the router node selector
func (w *ArksDisaggAppWrapper) WithRouterNodeSelector(selector map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.NodeSelector = selector
	return w
}

// WithPrefillTerminationGracePeriod sets the prefill termination grace period
func (w *ArksDisaggAppWrapper) WithPrefillTerminationGracePeriod(seconds int64) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.TerminationGracePeriodSeconds = &seconds
	return w
}

// WithDecodeTerminationGracePeriod sets the decode termination grace period
func (w *ArksDisaggAppWrapper) WithDecodeTerminationGracePeriod(seconds int64) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.TerminationGracePeriodSeconds = &seconds
	return w
}

// WithRouterTerminationGracePeriod sets the router termination grace period
func (w *ArksDisaggAppWrapper) WithRouterTerminationGracePeriod(seconds int64) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.TerminationGracePeriodSeconds = &seconds
	return w
}

// WithPrefillTolerations sets the prefill tolerations
func (w *ArksDisaggAppWrapper) WithPrefillTolerations(tolerations []corev1.Toleration) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.Tolerations = tolerations
	return w
}

// WithDecodeTolerations sets the decode tolerations
func (w *ArksDisaggAppWrapper) WithDecodeTolerations(tolerations []corev1.Toleration) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.Tolerations = tolerations
	return w
}

// WithRouterTolerations sets the router tolerations
func (w *ArksDisaggAppWrapper) WithRouterTolerations(tolerations []corev1.Toleration) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.Tolerations = tolerations
	return w
}

// WithPrefillLabels sets the prefill pod labels
func (w *ArksDisaggAppWrapper) WithPrefillLabels(labels map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.Labels = labels
	return w
}

// WithDecodeLabels sets the decode pod labels
func (w *ArksDisaggAppWrapper) WithDecodeLabels(labels map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.Labels = labels
	return w
}

// WithRouterLabels sets the router pod labels
func (w *ArksDisaggAppWrapper) WithRouterLabels(labels map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.Labels = labels
	return w
}

// WithPrefillAnnotations sets the prefill pod annotations
func (w *ArksDisaggAppWrapper) WithPrefillAnnotations(annotations map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.Annotations = annotations
	return w
}

// WithDecodeAnnotations sets the decode pod annotations
func (w *ArksDisaggAppWrapper) WithDecodeAnnotations(annotations map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.Annotations = annotations
	return w
}

// WithRouterAnnotations sets the router pod annotations
func (w *ArksDisaggAppWrapper) WithRouterAnnotations(annotations map[string]string) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.Annotations = annotations
	return w
}

// WithPrefillStartupProbe sets the prefill startup probe
func (w *ArksDisaggAppWrapper) WithPrefillStartupProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.StartupProbe = probe
	return w
}

// WithDecodeStartupProbe sets the decode startup probe
func (w *ArksDisaggAppWrapper) WithDecodeStartupProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.StartupProbe = probe
	return w
}

// WithRouterStartupProbe sets the router startup probe
func (w *ArksDisaggAppWrapper) WithRouterStartupProbe(probe *corev1.Probe) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.StartupProbe = probe
	return w
}

// WithPrefillAffinity sets the prefill affinity
func (w *ArksDisaggAppWrapper) WithPrefillAffinity(affinity *corev1.Affinity) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.Affinity = affinity
	return w
}

// WithDecodeAffinity sets the decode affinity
func (w *ArksDisaggAppWrapper) WithDecodeAffinity(affinity *corev1.Affinity) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.Affinity = affinity
	return w
}

// WithRouterAffinity sets the router affinity
func (w *ArksDisaggAppWrapper) WithRouterAffinity(affinity *corev1.Affinity) *ArksDisaggAppWrapper {
	w.Spec.Router.InstanceSpec.Affinity = affinity
	return w
}

// WithPrefillSecurityContext sets the prefill container security context
func (w *ArksDisaggAppWrapper) WithPrefillSecurityContext(ctx *corev1.SecurityContext) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.SecurityContext = ctx
	return w
}

// WithDecodeSecurityContext sets the decode container security context
func (w *ArksDisaggAppWrapper) WithDecodeSecurityContext(ctx *corev1.SecurityContext) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.SecurityContext = ctx
	return w
}

// WithPrefillPodSecurityContext sets the prefill pod security context
func (w *ArksDisaggAppWrapper) WithPrefillPodSecurityContext(ctx *corev1.PodSecurityContext) *ArksDisaggAppWrapper {
	w.Spec.Prefill.InstanceSpec.PodSecurityContext = ctx
	return w
}

// WithDecodePodSecurityContext sets the decode pod security context
func (w *ArksDisaggAppWrapper) WithDecodePodSecurityContext(ctx *corev1.PodSecurityContext) *ArksDisaggAppWrapper {
	w.Spec.Decode.InstanceSpec.PodSecurityContext = ctx
	return w
}

// BuildBasicArksDisaggApp creates a minimal ArksDisaggregatedApplication for testing
// Uses mock commands (sleep infinity) by default for e2e testing with mock images
// Also sets exec-based readiness probes that always succeed for mock containers
func BuildBasicArksDisaggApp(name, namespace string) *ArksDisaggAppWrapper {
	mockCmd := []string{"sleep", "infinity"}
	// Use exec probe that always succeeds (true command)
	mockReadinessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"true"},
			},
		},
		InitialDelaySeconds: 1,
		PeriodSeconds:       5,
	}
	return &ArksDisaggAppWrapper{
		arksv1.ArksDisaggregatedApplication{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "arks.ai/v1",
				Kind:       "ArksDisaggregatedApplication",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: arksv1.ArksDisaggregatedApplicationSpec{
				Runtime:      string(arksv1.ArksRuntimeSGLang),
				RouterImage:  DefaultRouterImage,
				RuntimeImage: DefaultRuntimeImage,
				Model:        corev1.LocalObjectReference{Name: "test-model"},
				Router: arksv1.ArksDisaggregatedRouter{
					Replicas:        ptr.To(int32(1)),
					CommandOverride: mockCmd,
					InstanceSpec: arksv1.ArksInstanceSpec{
						ReadinessProbe: mockReadinessProbe,
					},
				},
				Prefill: arksv1.ArksDisaggregatedWorkload{
					Replicas:              ptr.To(int32(1)),
					Size:                  1,
					LeaderCommandOverride: mockCmd,
					WorkerCommandOverride: mockCmd,
					InstanceSpec: arksv1.ArksInstanceSpec{
						ReadinessProbe: mockReadinessProbe,
					},
				},
				Decode: arksv1.ArksDisaggregatedWorkload{
					Replicas:              ptr.To(int32(1)),
					Size:                  1,
					LeaderCommandOverride: mockCmd,
					WorkerCommandOverride: mockCmd,
					InstanceSpec: arksv1.ArksInstanceSpec{
						ReadinessProbe: mockReadinessProbe,
					},
				},
			},
		},
	}
}

// BuildRealArksDisaggApp creates an ArksDisaggregatedApplication for real GPU cluster testing
// This does NOT use mock commands - it lets the actual sglang/vllm runtime run
// Use this for testing on clusters with real GPU nodes
func BuildRealArksDisaggApp(name, namespace string) *ArksDisaggAppWrapper {
	return &ArksDisaggAppWrapper{
		arksv1.ArksDisaggregatedApplication{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "arks.ai/v1",
				Kind:       "ArksDisaggregatedApplication",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: arksv1.ArksDisaggregatedApplicationSpec{
				Runtime: string(arksv1.ArksRuntimeSGLang),
				// Images should be set via WithRuntimeImage/WithRouterImage
				Router: arksv1.ArksDisaggregatedRouter{
					Replicas: ptr.To(int32(1)),
				},
				Prefill: arksv1.ArksDisaggregatedWorkload{
					Replicas: ptr.To(int32(1)),
					Size:     1,
				},
				Decode: arksv1.ArksDisaggregatedWorkload{
					Replicas: ptr.To(int32(1)),
					Size:     1,
				},
			},
		},
	}
}
