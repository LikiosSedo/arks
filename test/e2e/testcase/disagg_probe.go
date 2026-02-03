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
	"k8s.io/apimachinery/pkg/util/intstr"

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

// RunDisaggProbeTestCases runs probe configuration tests
func RunDisaggProbeTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Probes", ginkgo.Label("probe"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should add readiness probe to prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-probe-prefill-readiness", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add readiness probe
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.ReadinessProbe = &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 10,
					PeriodSeconds:       5,
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update readiness probe for prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-probe-prefill-readiness-update", f.Namespace).
				WithPrefillReadinessProbe(&corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 10,
					PeriodSeconds:       5,
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update readiness probe
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.ReadinessProbe = &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/ready",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 15,
					PeriodSeconds:       10,
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add liveness probe to prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-probe-prefill-liveness", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add liveness probe
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.LivenessProbe = &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 30,
					PeriodSeconds:       10,
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update liveness probe for prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-probe-prefill-liveness-update", f.Namespace).
				WithPrefillLivenessProbe(&corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 30,
					PeriodSeconds:       10,
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update liveness probe
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.LivenessProbe = &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 20,
					PeriodSeconds:       5,
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add readiness probe to decode", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-probe-decode-readiness", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add readiness probe
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.ReadinessProbe = &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/health",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 10,
					PeriodSeconds:       5,
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add liveness probe to decode", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-probe-decode-liveness", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add liveness probe
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.LivenessProbe = &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(8080),
						},
					},
					InitialDelaySeconds: 30,
					PeriodSeconds:       10,
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})
	})
}
