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

// RunDisaggRollingUpdateTestCases runs rolling update tests for ArksDisaggregatedApplication
func RunDisaggRollingUpdateTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Rolling Update", ginkgo.Label("rolling-update"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should rolling update router role independently", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-rolling-router", f.Namespace).
				WithRouterReplicas(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update router resources to trigger rolling update
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.InstanceSpec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("200m"),
					},
				}
			})

			// Verify rolling update completes
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRouterReplicas(app, 2)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should rolling update prefill role independently", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-rolling-prefill", f.Namespace).
				WithPrefillReplicas(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update prefill env to trigger rolling update
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "ROLLING_UPDATE_TEST", Value: "prefill"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should rolling update decode role independently", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-rolling-decode", f.Namespace).
				WithDecodeReplicas(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update decode env to trigger rolling update
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "ROLLING_UPDATE_TEST", Value: "decode"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectDecodeReplicas(app, 2)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should rolling update all roles together", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-rolling-all", f.Namespace).
				WithRouterReplicas(2).
				WithPrefillReplicas(2).
				WithDecodeReplicas(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update all roles at once
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				globalEnv := []corev1.EnvVar{{Name: "GLOBAL_UPDATE", Value: "true"}}
				a.Spec.Router.InstanceSpec.Env = globalEnv
				a.Spec.Prefill.InstanceSpec.Env = globalEnv
				a.Spec.Decode.InstanceSpec.Env = globalEnv
			})

			// All roles should update and become ready
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRouterReplicas(app, 2)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectDecodeReplicas(app, 2)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should rolling update when resources change", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-rolling-resources", f.Namespace).
				WithPrefillReplicas(2).
				WithPrefillResources(corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update resources
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectRollingUpdateComplete(app)
		})
	})
}
