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
	"k8s.io/utils/ptr"

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

// RunDisaggScalingTestCases runs scaling tests for ArksDisaggregatedApplication
func RunDisaggScalingTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Scaling", ginkgo.Label("scaling"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			// Create mock model for each test
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should scale up router replicas", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-scale-router-up", f.Namespace).
				WithRouterReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRouterReplicas(app, 1)

			// Scale up router
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.Replicas = ptr.To(int32(2))
			})
			f.ExpectRouterReplicas(app, 2)
		})

		ginkgo.It("should scale down router replicas", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-scale-router-down", f.Namespace).
				WithRouterReplicas(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRouterReplicas(app, 2)

			// Scale down router
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.Replicas = ptr.To(int32(1))
			})
			f.ExpectRouterReplicas(app, 1)
		})

		ginkgo.It("should scale up prefill replicas", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-scale-prefill-up", f.Namespace).
				WithPrefillReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 1)

			// Scale up prefill
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.Replicas = ptr.To(int32(3))
			})
			f.ExpectPrefillReplicas(app, 3)
		})

		ginkgo.It("should scale down prefill replicas", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-scale-prefill-down", f.Namespace).
				WithPrefillReplicas(3).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 3)

			// Scale down prefill
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.Replicas = ptr.To(int32(1))
			})
			f.ExpectPrefillReplicas(app, 1)
		})

		ginkgo.It("should scale up decode replicas", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-scale-decode-up", f.Namespace).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectDecodeReplicas(app, 1)

			// Scale up decode
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.Replicas = ptr.To(int32(2))
			})
			f.ExpectDecodeReplicas(app, 2)
		})

		ginkgo.It("should scale down decode replicas", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-scale-decode-down", f.Namespace).
				WithDecodeReplicas(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectDecodeReplicas(app, 2)

			// Scale down decode
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.Replicas = ptr.To(int32(1))
			})
			f.ExpectDecodeReplicas(app, 1)
		})

		ginkgo.It("should scale all roles simultaneously", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-scale-all", f.Namespace).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Scale up all roles
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.Replicas = ptr.To(int32(2))
				a.Spec.Prefill.Replicas = ptr.To(int32(2))
				a.Spec.Decode.Replicas = ptr.To(int32(2))
			})

			f.ExpectRouterReplicas(app, 2)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectDecodeReplicas(app, 2)

			// Scale down all roles
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.Replicas = ptr.To(int32(1))
				a.Spec.Prefill.Replicas = ptr.To(int32(1))
				a.Spec.Decode.Replicas = ptr.To(int32(1))
			})

			f.ExpectRouterReplicas(app, 1)
			f.ExpectPrefillReplicas(app, 1)
			f.ExpectDecodeReplicas(app, 1)
		})
	})
}
