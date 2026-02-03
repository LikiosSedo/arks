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
	"github.com/arks-ai/arks/test/wrappers"
)

// RunDisaggBasicTestCases runs basic CRUD tests for ArksDisaggregatedApplication
func RunDisaggBasicTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Basic", ginkgo.Label("basic"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			// Create mock model for each test
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should create ArksDisaggregatedApplication successfully", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-basic-create", f.Namespace).Obj()

			// Create
			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify RBGS created
			f.ExpectRBGSExists(app)
			f.ExpectRBGSReady(app)

			// Verify all roles have expected replicas
			f.ExpectRouterReplicas(app, 1)
			f.ExpectPrefillReplicas(app, 1)
			f.ExpectDecodeReplicas(app, 1)
		})

		ginkgo.It("should delete ArksDisaggregatedApplication successfully", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-basic-delete", f.Namespace).Obj()

			// Create
			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Delete
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)

			// Verify underlying resources are cleaned up
			f.ExpectRBGSDeleted(app)
		})

		ginkgo.It("should create ArksDisaggregatedApplication with custom replicas", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-basic-replicas", f.Namespace).
				WithRouterReplicas(2).
				WithPrefillReplicas(2).
				WithDecodeReplicas(3).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify replicas
			f.ExpectRouterReplicas(app, 2)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectDecodeReplicas(app, 3)
		})

		ginkgo.It("should verify RBGS has correct roles", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-basic-roles", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify RBGS has 3 roles: scheduler, prefill, decode
			f.ExpectRBGSRoleCount(app, 3)
			f.ExpectRBGRoleExists(app, "scheduler")
			f.ExpectRBGRoleExists(app, "prefill")
			f.ExpectRBGRoleExists(app, "decode")
		})
	})
}
