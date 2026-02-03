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

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

const (
	// Alternative mock images for testing image updates (using siflow registry)
	// These images should be long-running containers that stay alive
	AlternativeMockImage  = "registry-cn-shanghai.siflow.cn/k8s/coredns:v1.10.1"
	AlternativeMockImage2 = "registry-cn-shanghai.siflow.cn/k8s/etcd:3.5.12-0"
)

// RunDisaggImageTestCases runs image update tests
func RunDisaggImageTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Image", ginkgo.Label("image"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should update runtime image", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-image-runtime", f.Namespace).
				WithRuntimeImage(wrappers.DefaultRuntimeImage).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update runtime image
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.RuntimeImage = AlternativeMockImage
			})

			// This should trigger rolling update for prefill and decode
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update router image", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-image-router", f.Namespace).
				WithRouterImage(wrappers.DefaultRouterImage).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update router image
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.RouterImage = AlternativeMockImage
			})

			// This should trigger rolling update for router
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update both runtime and router images", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-image-both", f.Namespace).
				WithRouterImage(wrappers.DefaultRouterImage).
				WithRuntimeImage(wrappers.DefaultRuntimeImage).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update both images
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.RouterImage = AlternativeMockImage
				a.Spec.RuntimeImage = AlternativeMockImage2
			})

			// All roles should update
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should handle multiple image updates sequentially", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-image-sequential", f.Namespace).
				WithRuntimeImage(wrappers.DefaultRuntimeImage).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// First update
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.RuntimeImage = AlternativeMockImage
			})
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)

			// Second update
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.RuntimeImage = AlternativeMockImage2
			})
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})
	})
}
