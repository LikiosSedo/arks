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

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

// RunDisaggEnvTestCases runs environment variable update tests
func RunDisaggEnvTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Environment Variables", ginkgo.Label("env"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should add environment variables to router", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-env-router-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add env vars
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "TEST_ENV", Value: "router_test"},
					{Name: "ANOTHER_ENV", Value: "value2"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update environment variables for router", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-env-router-update", f.Namespace).
				WithRouterEnv([]corev1.EnvVar{
					{Name: "INITIAL_ENV", Value: "initial"},
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update env vars
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Router.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "INITIAL_ENV", Value: "updated"},
					{Name: "NEW_ENV", Value: "new_value"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add environment variables to prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-env-prefill-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add env vars
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "PREFILL_ENV", Value: "prefill_test"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update environment variables for prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-env-prefill-update", f.Namespace).
				WithPrefillEnv([]corev1.EnvVar{
					{Name: "INITIAL_ENV", Value: "initial"},
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update env vars
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "INITIAL_ENV", Value: "updated"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add environment variables to decode", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-env-decode-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add env vars
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "DECODE_ENV", Value: "decode_test"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update environment variables for decode", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-env-decode-update", f.Namespace).
				WithDecodeEnv([]corev1.EnvVar{
					{Name: "INITIAL_ENV", Value: "initial"},
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update env vars
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.Env = []corev1.EnvVar{
					{Name: "INITIAL_ENV", Value: "updated"},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update env vars for all roles simultaneously", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-env-all", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add env vars to all roles
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				globalEnv := []corev1.EnvVar{
					{Name: "GLOBAL_ENV", Value: "global_value"},
				}
				a.Spec.Router.InstanceSpec.Env = globalEnv
				a.Spec.Prefill.InstanceSpec.Env = globalEnv
				a.Spec.Decode.InstanceSpec.Env = globalEnv
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})
	})
}
