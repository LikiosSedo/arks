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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	rbgv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

var _ = Describe("ArksApplication Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		arksapplication := &arksv1.ArksApplication{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ArksApplication")
			err := k8sClient.Get(ctx, typeNamespacedName, arksapplication)
			if err != nil && errors.IsNotFound(err) {
				resource := &arksv1.ArksApplication{
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
			resource := &arksv1.ArksApplication{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ArksApplication")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ArksApplicationReconciler{
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

	Context("Backend selection and compatibility", func() {
		It("should default to LWS backend when not specified", func() {
			app := &arksv1.ArksApplication{
				Spec: arksv1.ArksApplicationSpec{
					// Backend not specified
					Replicas: 3,
				},
			}

			// Simulate backend selection logic
			backend := app.Spec.Backend
			if backend == "" {
				backend = arksv1.ArksBackendLWS
			}

			Expect(backend).To(Equal(arksv1.ArksBackendLWS))
		})

		It("should use LWS backend when explicitly specified", func() {
			app := &arksv1.ArksApplication{
				Spec: arksv1.ArksApplicationSpec{
					Backend:  arksv1.ArksBackendLWS,
					Replicas: 3,
				},
			}

			Expect(app.Spec.Backend).To(Equal(arksv1.ArksBackendLWS))
		})

		It("should use RBG backend when explicitly specified", func() {
			app := &arksv1.ArksApplication{
				Spec: arksv1.ArksApplicationSpec{
					Backend:  arksv1.ArksBackendRBG,
					Replicas: 3,
				},
			}

			Expect(app.Spec.Backend).To(Equal(arksv1.ArksBackendRBG))
		})
	})

	Context("RBG generation", func() {
		var (
			testApp   *arksv1.ArksApplication
			testModel *arksv1.ArksModel
		)

		BeforeEach(func() {
			testApp = &arksv1.ArksApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rbg-app",
					Namespace: "default",
				},
				Spec: arksv1.ArksApplicationSpec{
					Backend:      arksv1.ArksBackendRBG,
					Replicas:     3,
					Size:         2,
					Runtime:      "vllm",
					RuntimeImage: "vllm/vllm-openai:v0.8.2",
					Model: corev1.LocalObjectReference{
						Name: "test-model",
					},
				},
			}

			testModel = &arksv1.ArksModel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-model",
				},
				Spec: arksv1.ArksModelSpec{
					Model: "test-model",
					Storage: &arksv1.ArksModelStorage{
						PVC: &arksv1.ArksModelStoragePVC{
							Name: "model-pvc",
							Spec: corev1.PersistentVolumeClaimSpec{},
						},
					},
				},
			}
		})

		It("should generate valid RBGS resource", func() {
			rbgs, err := generateRBGS(testApp, testModel)

			Expect(err).NotTo(HaveOccurred())
			Expect(rbgs).NotTo(BeNil())
			Expect(rbgs.Name).To(Equal(testApp.Name))
			Expect(rbgs.Namespace).To(Equal(testApp.Namespace))
			Expect(*rbgs.Spec.Replicas).To(Equal(int32(testApp.Spec.Replicas)))
		})

		It("should configure rolling update strategy for RBG", func() {
			rbgs, err := generateRBGS(testApp, testModel)

			Expect(err).NotTo(HaveOccurred())
			Expect(rbgs.Spec.Template.Roles).To(HaveLen(1))

			role := rbgs.Spec.Template.Roles[0]
			Expect(role.RolloutStrategy).NotTo(BeNil())
			Expect(role.RolloutStrategy.Type).To(Equal(rbgv1alpha1.RollingUpdateStrategyType))
			Expect(role.RolloutStrategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(1)))
			Expect(role.RolloutStrategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(0)))
		})

		It("should use LeaderWorkerSet as workload type in RBG", func() {
			rbgs, err := generateRBGS(testApp, testModel)

			Expect(err).NotTo(HaveOccurred())
			role := rbgs.Spec.Template.Roles[0]
			Expect(role.Workload.Kind).To(Equal("LeaderWorkerSet"))
			Expect(role.Workload.APIVersion).To(Equal("leaderworkerset.x-k8s.io/v1"))
		})

		It("should handle negative replicas correctly", func() {
			testApp.Spec.Replicas = -1
			rbgs, err := generateRBGS(testApp, testModel)

			Expect(err).NotTo(HaveOccurred())
			Expect(*rbgs.Spec.Replicas).To(Equal(int32(0)))
		})
	})

	Context("LWS backward compatibility", func() {
		var (
			testApp   *arksv1.ArksApplication
			testModel *arksv1.ArksModel
		)

		BeforeEach(func() {
			testApp = &arksv1.ArksApplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-lws-app",
					Namespace: "default",
				},
				Spec: arksv1.ArksApplicationSpec{
					// Backend not specified - should default to LWS
					Replicas:     5,
					Size:         3,
					Runtime:      "sglang",
					RuntimeImage: "lmsysorg/sglang:v0.4.5",
					Model: corev1.LocalObjectReference{
						Name: "test-model",
					},
					InstanceSpec: arksv1.ArksInstanceSpec{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("16"),
								corev1.ResourceMemory: resource.MustParse("32Gi"),
							},
						},
						NodeSelector: map[string]string{
							"node.kubernetes.io/gpu": "true",
						},
					},
				},
			}

			testModel = &arksv1.ArksModel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-model",
				},
				Spec: arksv1.ArksModelSpec{
					Model: "test-model",
					Storage: &arksv1.ArksModelStorage{
						PVC: &arksv1.ArksModelStoragePVC{
							Name: "model-pvc",
							Spec: corev1.PersistentVolumeClaimSpec{},
						},
					},
				},
			}
		})

		It("should generate valid LWS resource for backward compatibility", func() {
			lws, err := generateLws(testApp, testModel)

			Expect(err).NotTo(HaveOccurred())
			Expect(lws).NotTo(BeNil())
			Expect(lws.Name).To(Equal(testApp.Name))
			Expect(lws.Namespace).To(Equal(testApp.Namespace))
			Expect(*lws.Spec.Replicas).To(Equal(int32(testApp.Spec.Replicas)))
		})

		It("should preserve all LWS fields correctly", func() {
			lws, err := generateLws(testApp, testModel)

			Expect(err).NotTo(HaveOccurred())
			Expect(*lws.Spec.LeaderWorkerTemplate.Size).To(Equal(int32(testApp.Spec.Size)))

			leaderPod := lws.Spec.LeaderWorkerTemplate.LeaderTemplate
			Expect(leaderPod).NotTo(BeNil())
			Expect(leaderPod.Spec.NodeSelector).To(Equal(testApp.Spec.InstanceSpec.NodeSelector))

			if len(leaderPod.Spec.Containers) > 0 {
				container := leaderPod.Spec.Containers[0]
				Expect(container.Resources).To(Equal(testApp.Spec.InstanceSpec.Resources))
			}
		})

		It("should support scaling for both LWS and RBG backends", func() {
			// Test LWS scaling
			testApp.Spec.Backend = arksv1.ArksBackendLWS
			initialReplicas := testApp.Spec.Replicas
			testApp.Spec.Replicas = 10

			lws, err := generateLws(testApp, testModel)
			Expect(err).NotTo(HaveOccurred())
			Expect(*lws.Spec.Replicas).To(Equal(int32(10)))

			// Test RBG scaling
			testApp.Spec.Backend = arksv1.ArksBackendRBG
			testApp.Spec.Replicas = 8

			rbgs, err := generateRBGS(testApp, testModel)
			Expect(err).NotTo(HaveOccurred())
			Expect(*rbgs.Spec.Replicas).To(Equal(int32(8)))

			_ = initialReplicas // Use variable to avoid unused warning
		})
	})
})
