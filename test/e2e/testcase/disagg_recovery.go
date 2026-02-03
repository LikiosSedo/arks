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

	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

// RunDisaggRecoveryTestCases runs pod restart/recovery tests
func RunDisaggRecoveryTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Recovery", ginkgo.Label("recovery"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should recover when router pod is deleted", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-recovery-router", f.Namespace).
				WithRouterReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRouterReplicas(app, 1)

			// Delete a router pod
			err := utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "router")
			gomega.Expect(err).Should(gomega.Succeed())

			// Should recover
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRouterReplicas(app, 1)
		})

		ginkgo.It("should recover when prefill pod is deleted", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-recovery-prefill", f.Namespace).
				WithPrefillReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 1)

			// Delete a prefill pod
			err := utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "prefill")
			gomega.Expect(err).Should(gomega.Succeed())

			// Should recover
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 1)
		})

		ginkgo.It("should recover when decode pod is deleted", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-recovery-decode", f.Namespace).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectDecodeReplicas(app, 1)

			// Delete a decode pod
			err := utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "decode")
			gomega.Expect(err).Should(gomega.Succeed())

			// Should recover
			f.ExpectArksDisaggAppReady(app)
			f.ExpectDecodeReplicas(app, 1)
		})

		ginkgo.It("should recover when multiple pods are deleted", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-recovery-multiple", f.Namespace).
				WithRouterReplicas(2).
				WithPrefillReplicas(2).
				WithDecodeReplicas(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			// Explicitly wait for all replicas to be ready before deleting
			f.ExpectRouterReplicas(app, 2)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectDecodeReplicas(app, 2)

			// Delete pods from multiple roles
			err := utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "router")
			gomega.Expect(err).Should(gomega.Succeed())

			err = utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "prefill")
			gomega.Expect(err).Should(gomega.Succeed())

			err = utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "decode")
			gomega.Expect(err).Should(gomega.Succeed())

			// All should recover
			f.ExpectArksDisaggAppReady(app)
			f.ExpectRouterReplicas(app, 2)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectDecodeReplicas(app, 2)
		})

		ginkgo.It("should recover multiple times", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-recovery-repeated", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 1) // Ensure prefill pods exist before deletion

			// First deletion
			err := utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "prefill")
			gomega.Expect(err).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 1) // Wait for recovery before next deletion

			// Second deletion
			err = utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "prefill")
			gomega.Expect(err).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 1) // Wait for recovery before next deletion

			// Third deletion
			err = utils.DeleteRolePod(f.Ctx, f.Client, f.Namespace, app.Name, "prefill")
			gomega.Expect(err).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, 1) // Verify final recovery
		})
	})
}
