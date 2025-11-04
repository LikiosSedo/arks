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

package controller

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

var _ = Describe("ArksDisaggregatedapplication Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		arksdisaggregatedapplication := &arksv1.ArksDisaggregatedApplication{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ArksDisaggregatedapplication")
			err := k8sClient.Get(ctx, typeNamespacedName, arksdisaggregatedapplication)
			if err != nil && errors.IsNotFound(err) {
				resource := &arksv1.ArksDisaggregatedApplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &arksv1.ArksDisaggregatedApplication{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ArksDisaggregatedapplication")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ArksDisaggregatedApplicationReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("generateDisaggregatedRBGS", func() {
		var (
			reconciler  *ArksDisaggregatedApplicationReconciler
			application *arksv1.ArksDisaggregatedApplication
			model       *arksv1.ArksModel
		)

		BeforeEach(func() {
			reconciler = &ArksDisaggregatedApplicationReconciler{}
			prefillReplicas := int32(3)
			decodeReplicas := int32(2)

			application = &arksv1.ArksDisaggregatedApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: arksv1.ArksDisaggregatedApplicationSpec{
					Backend:      arksv1.ArksBackendRBG,
					Runtime:      string(arksv1.ArksRuntimeSGLang),
					RuntimeImage: "example/runtime:latest",
					Model:        corev1.LocalObjectReference{Name: "test-model"},
					Prefill: arksv1.ArksDisaggregatedWorkload{
						Replicas: &prefillReplicas,
						Size:     4,
						InstanceSpec: arksv1.ArksInstanceSpec{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("16"),
									corev1.ResourceMemory: resource.MustParse("32Gi"),
								},
							},
							Env: []corev1.EnvVar{
								{Name: "TEST_ENV", Value: "prefill"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "extra", MountPath: "/data"},
							},
							Volumes: []corev1.Volume{
								{
									Name: "extra",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler:        corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(8081)}},
								InitialDelaySeconds: 10,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/live", Port: intstr.FromInt(8081)}},
							},
						},
					},
					Decode: arksv1.ArksDisaggregatedWorkload{
						Replicas: &decodeReplicas,
						Size:     2,
						InstanceSpec: arksv1.ArksInstanceSpec{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("8"),
									corev1.ResourceMemory: resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			}

			model = &arksv1.ArksModel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-model",
					Namespace: "default",
				},
				Spec: arksv1.ArksModelSpec{
					Storage: &arksv1.ArksModelStorage{
						PVC: &arksv1.ArksModelStoragePVC{
							Name: "model-pvc",
							Spec: corev1.PersistentVolumeClaimSpec{},
						},
					},
				},
			}
		})

		It("should generate RBGS with expected replicas and pod template for prefill", func() {
			rbgs, err := reconciler.generateDisaggregatedRBGS(application, model, "prefill")
			Expect(err).NotTo(HaveOccurred())
			Expect(rbgs).NotTo(BeNil())
			Expect(*rbgs.Spec.Replicas).To(Equal(int32(3)))

			Expect(rbgs.Spec.Template.Roles).To(HaveLen(1))
			role := rbgs.Spec.Template.Roles[0]
			Expect(role.Name).To(Equal("prefill"))
			Expect(role.LeaderWorkerSet.Size).NotTo(BeNil())
			Expect(*role.LeaderWorkerSet.Size).To(Equal(int32(4)))

			Expect(role.Template.Spec.Containers).To(HaveLen(1))
			container := role.Template.Spec.Containers[0]
			Expect(container.Image).To(Equal("example/runtime:latest"))
			Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: "TEST_ENV", Value: "prefill"}))
			Expect(container.VolumeMounts).To(ContainElement(corev1.VolumeMount{Name: "extra", MountPath: "/data"}))
			Expect(container.ReadinessProbe).NotTo(BeNil())
			Expect(container.LivenessProbe).NotTo(BeNil())
		})

		It("should respect command overrides for worker pods", func() {
			override := []string{"/bin/runner", "--custom"}
			application.Spec.Decode.WorkerCommandOverride = override
			application.Spec.Decode.InstanceSpec.Env = []corev1.EnvVar{{Name: "MODE", Value: "decode"}}

			rbgs, err := reconciler.generateDisaggregatedRBGS(application, model, "decode")
			Expect(err).NotTo(HaveOccurred())

			role := rbgs.Spec.Template.Roles[0]
			Expect(role.Workload.Kind).To(Equal("LeaderWorkerSet"))
			Expect(role.LeaderWorkerSet.PatchWorkerTemplate.Raw).NotTo(BeEmpty())

			patch := &corev1.PodTemplateSpec{}
			Expect(json.Unmarshal(role.LeaderWorkerSet.PatchWorkerTemplate.Raw, patch)).To(Succeed())
			Expect(patch.Spec.Containers).To(HaveLen(1))
			Expect(patch.Spec.Containers[0].Command).To(Equal(override))
			Expect(patch.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "MODE", Value: "decode"}))
		})
	})
})
