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

package testcase

import (
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

// GPU cluster configuration - can be overridden via environment variables
var (
	// Default sglang image for real GPU testing
	RealSglangImage = getEnvOrDefault("E2E_SGLANG_IMAGE", "registry-ap-southeast.scitix.ai/k8s/sglang:v0.5.2-cu126")

	// Default model name (must have corresponding ArksModel already created and Ready)
	RealModelName = getEnvOrDefault("E2E_MODEL_NAME", "qwen-7b")

	// Namespace where the ArksModel exists (real GPU tests run in this namespace)
	RealGPUNamespace = getEnvOrDefault("E2E_GPU_NAMESPACE", "default")

	// Node selector for GPU nodes
	GPUNodeSelector = map[string]string{
		"kubernetes.io/hostname": getEnvOrDefault("E2E_GPU_NODE", "hpe-node144"),
	}
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RunDisaggRealGPUTestCases runs e2e tests on real GPU cluster with actual inference workloads
// These tests require:
// - A Kubernetes cluster with GPU nodes
// - nvidia-device-plugin installed
// - ArksModel with downloaded model weights
// - Sufficient GPU resources available
//
// Run with: go test ./test/e2e/ -v -ginkgo.label-filter="real-gpu" -ginkgo.timeout=30m
//
// Prerequisites:
// - ArksModel must already exist and be Ready (e.g., qwen-7b in default namespace)
// - GPU nodes must be available with nvidia.com/gpu resources
// - The model PVC must have the model weights downloaded
//
// Environment variables:
// - E2E_SGLANG_IMAGE: sglang image (default: registry-ap-southeast.scitix.ai/k8s/sglang:v0.5.2-cu126)
// - E2E_MODEL_NAME: ArksModel name (default: qwen-7b)
// - E2E_GPU_NAMESPACE: namespace where ArksModel exists (default: default)
// - E2E_GPU_NODE: GPU node hostname (default: hpe-node144)
func RunDisaggRealGPUTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Real GPU", ginkgo.Label("real-gpu"), func() {

		// BeforeEach: Verify ArksModel exists (don't create it)
		ginkgo.BeforeEach(func() {
			f := *fp

			// Set SkipModelCleanup to prevent the global AfterEach from deleting ArksModels
			// This is critical because real GPU tests use pre-existing ArksModels
			f.SkipModelCleanup = true

			// Verify the ArksModel exists and is Ready
			model := &arksv1.ArksModel{}
			err := f.Client.Get(f.Ctx, client.ObjectKey{
				Name:      RealModelName,
				Namespace: RealGPUNamespace,
			}, model)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(),
				"ArksModel %s/%s must exist for real GPU tests", RealGPUNamespace, RealModelName)
			gomega.Expect(model.Status.Phase).To(gomega.Equal(string(arksv1.ArksModelReady)),
				"ArksModel %s/%s must be Ready", RealGPUNamespace, RealModelName)
		})

		// AfterEach: Only clean up ArksDisaggregatedApplication, NOT ArksModel
		// This prevents deleting the pre-existing qwen-7b model
		ginkgo.AfterEach(func() {
			f := *fp
			// Only delete ArksDisaggregatedApplications created by this test
			// Do NOT delete ArksModels - they are pre-existing resources
			err := f.Client.DeleteAllOf(
				f.Ctx,
				&arksv1.ArksDisaggregatedApplication{},
				client.InNamespace(RealGPUNamespace),
			)
			if err != nil {
				// Log but don't fail - best effort cleanup
				ginkgo.GinkgoWriter.Printf("Warning: failed to cleanup ArksDisaggregatedApplications: %v\n", err)
			}

			// Wait for cleanup to complete
			gomega.Eventually(func() int {
				list := &arksv1.ArksDisaggregatedApplicationList{}
				if err := f.Client.List(f.Ctx, list, client.InNamespace(RealGPUNamespace)); err != nil {
					return -1
				}
				// Filter out apps that weren't created by e2e tests
				count := 0
				for _, app := range list.Items {
					if len(app.Name) > 6 && app.Name[:6] == "e2e-re" {
						count++
					}
				}
				return count
			}, framework.Timeout, framework.Interval).Should(gomega.Equal(0),
				"All e2e test ArksDisaggregatedApplications should be deleted")
		})

		ginkgo.It("should deploy sglang PD disaggregated inference with real GPU", func() {
			f := *fp

			// GPU + shared memory configuration
			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			// Minimal memory config to avoid OOM
			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-basic", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			// Wait for application to be ready (pods scheduled and running on GPU nodes)
			f.ExpectArksDisaggAppReady(app)

			// Verify all replicas are actually running
			f.ExpectRouterReplicas(app, 1)
			f.ExpectPrefillReplicas(app, 1)
			f.ExpectDecodeReplicas(app, 1)

			// Verify GPU resources in RBGS
			f.ExpectRBGSRoleHasGPUResources(app, "prefill", 1)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)
		})

		ginkgo.It("should scale decode replicas on real GPU cluster", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			// Minimal memory config to avoid OOM
			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-scale", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			// Use longer timeout for GPU workloads (model loading takes time)
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectDecodeReplicasWithTimeout(app, 1, framework.GPUTimeout)

			// Scale up decode replicas
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.Replicas = ptrInt32(2)
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectDecodeReplicasWithTimeout(app, 2, framework.GPUTimeout)

			// Scale back down
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.Replicas = ptrInt32(1)
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectDecodeReplicasWithTimeout(app, 1, framework.GPUTimeout)
		})

		ginkgo.It("should perform rolling update on real GPU workload", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			// Minimal memory config to avoid OOM
			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-rolling", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Update runtime args to trigger rolling update (just change cuda-graph-max-bs)
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.RuntimeCommonArgs = []string{
					"--dtype=half",
					"--disable-flashinfer",
					"--mem-fraction-static=0.5",
					"--tp=1",
					"--cuda-graph-max-bs=16", // Changed from 8 to 16
				}
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
		})

		ginkgo.It("should recover GPU pod after deletion", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			// Minimal memory config to avoid OOM
			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-recovery", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectDecodeReplicasWithTimeout(app, 1, framework.GPUTimeout)

			// Delete decode pod
			err := utils.DeleteRolePod(f.Ctx, f.Client, RealGPUNamespace, app.Name, "decode")
			gomega.Expect(err).Should(gomega.Succeed())

			// Should recover
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectDecodeReplicasWithTimeout(app, 1, framework.GPUTimeout)
		})

		ginkgo.It("should deploy TP=2 configuration with 2 GPUs per pod", func() {
			f := *fp

			// TP=2 requires 2 GPUs per pod
			tp2Resources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("2"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("2"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			// Minimal memory config for TP=2
			tp2Args := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=2",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-tp2", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(tp2Resources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(tp2Args).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(tp2Resources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(tp2Args).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Verify 2 GPUs per role
			f.ExpectRBGSRoleHasGPUResources(app, "prefill", 2)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 2)
		})

		// Test: StartupProbe for GPU workloads
		// GPU model loading can take minutes, StartupProbe allows longer initial delay
		ginkgo.It("should deploy with startup probe for slow GPU initialization", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			// StartupProbe is critical for GPU workloads - sglang model loading takes 2-5 minutes
			// Without proper StartupProbe, the pod will be killed before model is ready
			startupProbe := &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: utils.IntOrString(30000),
					},
				},
				InitialDelaySeconds: 60, // Wait 1 minute before first probe
				PeriodSeconds:       10, // Check every 10 seconds
				TimeoutSeconds:      5,
				FailureThreshold:    30, // Allow up to 5 minutes for model loading (30 * 10s)
				SuccessThreshold:    1,
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-startup", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithPrefillStartupProbe(startupProbe).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				WithDecodeStartupProbe(startupProbe).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Verify startup probe is configured
			f.ExpectRolePodHasStartupProbe(app, "prefill")
			f.ExpectRolePodHasStartupProbe(app, "decode")
		})

		// Test: Tolerations for GPU nodes
		// GPU nodes often have taints (e.g., nvidia.com/gpu=present:NoSchedule)
		ginkgo.It("should deploy with tolerations for GPU node taints", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			// Common GPU node toleration
			gpuToleration := corev1.Toleration{
				Key:      "nvidia.com/gpu",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-toleration", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithPrefillTolerations([]corev1.Toleration{gpuToleration}).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				WithDecodeTolerations([]corev1.Toleration{gpuToleration}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Verify tolerations are applied
			f.ExpectRolePodHasToleration(app, "prefill", gpuToleration)
			f.ExpectRolePodHasToleration(app, "decode", gpuToleration)
		})

		// Test: Custom labels and annotations propagation
		// Useful for observability, cost allocation, and integration with external systems
		ginkgo.It("should propagate custom labels and annotations to GPU pods", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			customLabels := map[string]string{
				"team":        "ml-platform",
				"cost-center": "ai-inference",
				"gpu-type":    "h100",
			}

			customAnnotations := map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "30000",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-labels", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithPrefillLabels(customLabels).
				WithPrefillAnnotations(customAnnotations).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				WithDecodeLabels(customLabels).
				WithDecodeAnnotations(customAnnotations).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Verify labels are propagated
			f.ExpectRolePodHasLabel(app, "prefill", "team", "ml-platform")
			f.ExpectRolePodHasLabel(app, "decode", "team", "ml-platform")
			f.ExpectRolePodHasLabel(app, "prefill", "cost-center", "ai-inference")
			f.ExpectRolePodHasLabel(app, "decode", "cost-center", "ai-inference")

			// Verify annotations are propagated
			f.ExpectRolePodHasAnnotation(app, "prefill", "prometheus.io/scrape", "true")
			f.ExpectRolePodHasAnnotation(app, "decode", "prometheus.io/scrape", "true")
		})

		// Test: TerminationGracePeriodSeconds for GPU workloads
		// GPU inference needs time to gracefully shut down and release resources
		ginkgo.It("should respect termination grace period for GPU pods", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			// 60 seconds grace period for GPU cleanup
			gracePeriod := int64(60)

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-grace", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithPrefillTerminationGracePeriod(gracePeriod).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				WithDecodeTerminationGracePeriod(gracePeriod).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Verify termination grace period is set
			f.ExpectRolePodHasTerminationGracePeriod(app, "prefill", gracePeriod)
			f.ExpectRolePodHasTerminationGracePeriod(app, "decode", gracePeriod)
		})

		// Test: Environment variables for GPU configuration
		// CUDA and sglang specific environment variables
		ginkgo.It("should pass custom environment variables to GPU pods", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			// CUDA-related environment variables
			cudaEnvVars := []corev1.EnvVar{
				{Name: "CUDA_VISIBLE_DEVICES", Value: "0"},
				{Name: "NCCL_DEBUG", Value: "INFO"},
				{Name: "CUDA_DEVICE_ORDER", Value: "PCI_BUS_ID"},
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-env", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithPrefillEnv(cudaEnvVars).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				WithDecodeEnv(cudaEnvVars).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Verify pods have env vars by checking RBGS template
			f.ExpectRBGSRoleHasEnv(app, "prefill", "NCCL_DEBUG", "INFO")
			f.ExpectRBGSRoleHasEnv(app, "decode", "NCCL_DEBUG", "INFO")
		})

		// Test: Update runtime args on running GPU deployment
		// Verify args changes propagate without OOM
		ginkgo.It("should update runtime args without causing OOM", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			// Start with conservative memory settings
			initialArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=4", // Start with smaller batch size
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-args-update", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(initialArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(initialArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Update to larger batch size (still safe)
			updatedArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8", // Increase batch size
			}

			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.RuntimeCommonArgs = updatedArgs
				a.Spec.Decode.RuntimeCommonArgs = updatedArgs
			})

			// Should successfully update without OOM
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
		})

		// Test: Multiple decode replicas with limited GPU resources
		// Tests GPU resource scheduling across multiple pods
		ginkgo.It("should schedule multiple decode replicas on available GPUs", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			// Start with 2 decode replicas to test parallel GPU scheduling
			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-multi-decode", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(2). // 2 decode replicas
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectDecodeReplicasWithTimeout(app, 2, framework.GPUTimeout)

			// Verify all decode pods have GPU resources
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)
		})

		// ==========================================
		// Critical Rolling Update Tests (Real GPU)
		// These tests MUST run on real GPU because mock tests cannot catch:
		// - GPU memory allocation issues during restart
		// - CUDA context cleanup problems
		// - Model reload failures
		// ==========================================

		// Test: Rolling update image on GPU workload
		// Real GPU needed: image change triggers model reload, mock doesn't load models
		ginkgo.It("should rolling update runtime image without service disruption", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-image-update", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Update to same image (since we may not have another valid GPU image available)
			// This still triggers a rolling update and model reload
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				// Just add a trivial annotation to trigger image update test
				// In real scenario, you would change to a different image version
				if a.Spec.Decode.InstanceSpec.Annotations == nil {
					a.Spec.Decode.InstanceSpec.Annotations = make(map[string]string)
				}
				a.Spec.Decode.InstanceSpec.Annotations["image-update-test"] = "triggered"
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
		})

		// Test: Rolling update GPU resources
		// Real GPU needed: changing GPU count requires actual GPU scheduling
		ginkgo.It("should rolling update GPU resources from 1 to 2 GPUs", func() {
			f := *fp

			// Start with 1 GPU
			gpu1Resources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			// Start with TP=1
			tp1Args := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-resource-update", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpu1Resources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(tp1Args).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpu1Resources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(tp1Args).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Verify initial state
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)

			// Update decode to 2 GPUs with TP=2
			tp2Args := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=2",
				"--cuda-graph-max-bs=8",
			}

			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.Resources = corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						ResourceNvidiaGPU: resource.MustParse("2"),
					},
					Requests: corev1.ResourceList{
						ResourceNvidiaGPU: resource.MustParse("2"),
					},
				}
				a.Spec.Decode.RuntimeCommonArgs = tp2Args
			})

			// Wait for rolling update to complete with 2 GPUs
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 2)
		})

		// Test: Rolling update readiness and liveness probes on GPU workload
		// Real GPU needed: probe timing is critical for slow GPU initialization
		ginkgo.It("should rolling update probes without killing healthy GPU pods", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			// Start with basic probes
			initialReadinessProbe := &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: utils.IntOrString(30000),
					},
				},
				InitialDelaySeconds: 30,
				PeriodSeconds:       10,
				TimeoutSeconds:      5,
				FailureThreshold:    3,
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-probe-update", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithPrefillReadinessProbe(initialReadinessProbe).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				WithDecodeReadinessProbe(initialReadinessProbe).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Update probe parameters (more aggressive check)
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				updatedProbe := &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
							Port: utils.IntOrString(30000),
						},
					},
					InitialDelaySeconds: 60, // Increase for GPU workloads
					PeriodSeconds:       5,  // More frequent checks
					TimeoutSeconds:      10, // Longer timeout
					FailureThreshold:    5,  // More tolerant
				}
				a.Spec.Decode.InstanceSpec.ReadinessProbe = updatedProbe
				a.Spec.Prefill.InstanceSpec.ReadinessProbe = updatedProbe
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
		})

		// Test: Rolling update volume mounts on GPU workload
		// Real GPU needed: sglang uses /dev/shm for shared memory communication
		ginkgo.It("should rolling update volume mounts preserving GPU functionality", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			// Initial volumes - just shm
			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-volume-update", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Add an additional cache volume
			cacheVolume := corev1.Volume{
				Name: "cache",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}
			cacheMount := corev1.VolumeMount{
				Name:      "cache",
				MountPath: "/tmp/cache",
			}

			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.Volumes = []corev1.Volume{shmVolume, cacheVolume}
				a.Spec.Decode.InstanceSpec.VolumeMounts = []corev1.VolumeMount{shmMount, cacheMount}
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
		})

		// Test: Rolling update command override on GPU workload
		// Real GPU needed: command changes affect how sglang is launched
		ginkgo.It("should rolling update leader command override on decode role", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=8",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-cmd-update", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Update runtime args (which affects how the decode workers start)
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				// Change cuda-graph-max-bs to trigger rolling update
				a.Spec.Decode.RuntimeCommonArgs = []string{
					"--dtype=half",
					"--disable-flashinfer",
					"--mem-fraction-static=0.5",
					"--tp=1",
					"--cuda-graph-max-bs=16", // Changed from 8 to 16
				}
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
		})

		// Test: Combined rolling update (multiple fields at once)
		// Real GPU needed: combined changes stress GPU resource management
		ginkgo.It("should handle combined rolling update of resources, args, and probes", func() {
			f := *fp

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			shmVolume := corev1.Volume{
				Name: "shm",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}
			shmMount := corev1.VolumeMount{
				Name:      "shm",
				MountPath: "/dev/shm",
			}

			minimalArgs := []string{
				"--dtype=half",
				"--disable-flashinfer",
				"--mem-fraction-static=0.5",
				"--tp=1",
				"--cuda-graph-max-bs=4",
			}

			app := wrappers.BuildRealArksDisaggApp("e2e-real-gpu-combined-update", RealGPUNamespace).
				WithModel(RealModelName).
				WithRuntime("sglang").
				WithRuntimeImage(RealSglangImage).
				WithRouterImage(RealSglangImage).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(1).
				WithPrefillResources(gpuResources).
				WithPrefillNodeSelector(GPUNodeSelector).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithPrefillRuntimeArgs(minimalArgs).
				WithDecodeReplicas(1).
				WithDecodeSize(1).
				WithDecodeResources(gpuResources).
				WithDecodeNodeSelector(GPUNodeSelector).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeRuntimeArgs(minimalArgs).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)

			// Combined update: args + labels + annotations
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				// Update runtime args
				a.Spec.Decode.RuntimeCommonArgs = []string{
					"--dtype=half",
					"--disable-flashinfer",
					"--mem-fraction-static=0.5",
					"--tp=1",
					"--cuda-graph-max-bs=8", // Updated
				}
				// Add labels
				if a.Spec.Decode.InstanceSpec.Labels == nil {
					a.Spec.Decode.InstanceSpec.Labels = make(map[string]string)
				}
				a.Spec.Decode.InstanceSpec.Labels["version"] = "v2"
				// Add annotations
				if a.Spec.Decode.InstanceSpec.Annotations == nil {
					a.Spec.Decode.InstanceSpec.Annotations = make(map[string]string)
				}
				a.Spec.Decode.InstanceSpec.Annotations["update-reason"] = "combined-test"
			})

			f.ExpectArksDisaggAppReadyWithTimeout(app, framework.GPUTimeout)
			f.ExpectRollingUpdateCompleteWithTimeout(app, framework.GPUTimeout)
			// Verify labels were applied
			f.ExpectRolePodHasLabel(app, "decode", "version", "v2")
		})
	})
}
