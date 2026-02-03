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

// RunDisaggCommandTestCases runs command/args update tests
func RunDisaggCommandTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Commands", ginkgo.Label("command"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should add runtime args to prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-cmd-prefill-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add runtime args
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.RuntimeCommonArgs = []string{
					"--max-batch-size=64",
					"--timeout=30",
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update runtime args for prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-cmd-prefill-update", f.Namespace).
				WithPrefillRuntimeArgs([]string{
					"--max-batch-size=32",
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update runtime args
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.RuntimeCommonArgs = []string{
					"--max-batch-size=128",
					"--timeout=60",
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add runtime args to decode", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-cmd-decode-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add runtime args
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.RuntimeCommonArgs = []string{
					"--decode-batch-size=16",
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update runtime args for decode", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-cmd-decode-update", f.Namespace).
				WithDecodeRuntimeArgs([]string{
					"--decode-batch-size=16",
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update runtime args
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.RuntimeCommonArgs = []string{
					"--decode-batch-size=32",
					"--decode-timeout=120",
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add router args", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-cmd-router-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add router args
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.RouterArgs = []string{
					"--max-connections=1000",
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update router args", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-cmd-router-update", f.Namespace).
				WithRouterArgs([]string{
					"--max-connections=500",
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update router args
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.RouterArgs = []string{
					"--max-connections=2000",
					"--timeout=120",
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update args for all roles simultaneously", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-cmd-all", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update args for all roles
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.RouterArgs = []string{"--router-flag=true"}
				a.Spec.Prefill.RuntimeCommonArgs = []string{"--prefill-flag=true"}
				a.Spec.Decode.RuntimeCommonArgs = []string{"--decode-flag=true"}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})
	})
}
