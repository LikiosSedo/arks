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
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

const (
	// GPU resource name
	ResourceNvidiaGPU corev1.ResourceName = "nvidia.com/gpu"
	// RDMA resource name (for InfiniBand)
	ResourceRDMA corev1.ResourceName = "rdma/hca_shared_devices_all"
)

// RunDisaggGPUResourcesTestCases runs GPU resource configuration tests
// These tests verify GPU resource settings are correctly propagated to pods
// They can run on fake-nodes that accept GPU resource requests
func RunDisaggGPUResourcesTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication GPU Resources", ginkgo.Label("gpu-resources"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should create application with GPU resources for prefill and decode", func() {
			f := *fp
			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gpu-basic", f.Namespace).
				WithPrefillResources(gpuResources).
				WithDecodeResources(gpuResources).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify GPU resources are set in RBGS
			f.ExpectRBGSRoleHasGPUResources(app, "prefill", 1)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)
		})

		ginkgo.It("should create application with GPU and RDMA resources", func() {
			f := *fp
			// Full GPU + RDMA configuration like production
			gpuRdmaResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
					ResourceRDMA:      resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
					ResourceRDMA:      resource.MustParse("1"),
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gpu-rdma", f.Namespace).
				WithPrefillResources(gpuRdmaResources).
				WithDecodeResources(gpuRdmaResources).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify resources are propagated
			f.ExpectRBGSRoleHasGPUResources(app, "prefill", 1)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)
		})

		ginkgo.It("should update GPU resources and trigger rolling update", func() {
			f := *fp
			initialResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gpu-update", f.Namespace).
				WithPrefillResources(initialResources).
				WithDecodeResources(initialResources).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update GPU count (e.g., for TP=2)
			updatedResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("2"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("2"),
				},
			}

			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.Resources = updatedResources
				a.Spec.Decode.InstanceSpec.Resources = updatedResources
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)

			// Verify new GPU count
			f.ExpectRBGSRoleHasGPUResources(app, "prefill", 2)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 2)
		})

		ginkgo.It("should create application with shared memory volume for sglang", func() {
			f := *fp
			// Shared memory volume required by sglang
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

			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gpu-shm", f.Namespace).
				WithPrefillResources(gpuResources).
				WithPrefillVolumes([]corev1.Volume{shmVolume}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{shmMount}).
				WithDecodeResources(gpuResources).
				WithDecodeVolumes([]corev1.Volume{shmVolume}).
				WithDecodeVolumeMounts([]corev1.VolumeMount{shmMount}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
		})

		ginkgo.It("should create application with different GPU counts per role", func() {
			f := *fp
			// Prefill with 2 GPUs, Decode with 1 GPU
			prefillResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("2"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("2"),
				},
			}
			decodeResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gpu-asymmetric", f.Namespace).
				WithPrefillResources(prefillResources).
				WithDecodeResources(decodeResources).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify different GPU counts
			f.ExpectRBGSRoleHasGPUResources(app, "prefill", 2)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)
		})

		ginkgo.It("should create multi-GPU application with TP parallelism", func() {
			f := *fp
			// TP=4 configuration: 4 GPUs per pod, size=1 (single pod per group)
			tp4Resources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("4"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("4"),
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gpu-tp4", f.Namespace).
				WithPrefillResources(tp4Resources).
				WithPrefillSize(1).
				WithPrefillReplicas(1).
				WithDecodeResources(tp4Resources).
				WithDecodeSize(1).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			f.ExpectRBGSRoleHasGPUResources(app, "prefill", 4)
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 4)
		})

		// This test verifies GPU resources are maintained after scaling
		// Note: Pods will only become ready on clusters with GPU nodes
		// On non-GPU clusters, this test will verify resource propagation in RBGS
		ginkgo.It("should scale GPU workload and maintain GPU resources in RBGS", func() {
			f := *fp
			gpuResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
				Requests: corev1.ResourceList{
					ResourceNvidiaGPU: resource.MustParse("1"),
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gpu-scale", f.Namespace).
				WithPrefillResources(gpuResources).
				WithPrefillReplicas(1).
				WithDecodeResources(gpuResources).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify initial GPU resources
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)

			// Scale up decode replicas
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.Replicas = ptrInt32(2)
			})

			// Wait for RBGS to be updated with new replica count
			// Note: We don't wait for pods to be ready as they may not schedule on non-GPU nodes
			f.ExpectRBGRoleReplicas(app, "decode", 2)

			// GPU resources should still be maintained in RBGS
			f.ExpectRBGSRoleHasGPUResources(app, "decode", 1)
		})
	})
}

// ptrInt32 returns a pointer to the given int32 value
func ptrInt32(i int32) *int32 {
	return &i
}
