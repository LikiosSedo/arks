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
	"github.com/arks-ai/arks/test/wrappers"
)

// RunDisaggGangSchedulingTestCases runs gang scheduling (PodGroupPolicy) tests
func RunDisaggGangSchedulingTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Gang Scheduling", ginkgo.Label("gang"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should create application with KubeScheduling PodGroupPolicy", func() {
			f := *fp
			policy := &arksv1.PodGroupPolicy{
				PodGroupPolicySource: arksv1.PodGroupPolicySource{
					KubeScheduling: &arksv1.KubeSchedulingPodGroupPolicySource{
						ScheduleTimeoutSeconds: ptr.To(int32(120)),
					},
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gang-kube", f.Namespace).
				WithPodGroupPolicy(policy).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify RBGS has PodGroupPolicy
			f.ExpectRBGSHasPodGroupPolicy(app, "kubeScheduling")
		})

		ginkgo.It("should create application with VolcanoScheduling PodGroupPolicy", func() {
			f := *fp
			policy := &arksv1.PodGroupPolicy{
				PodGroupPolicySource: arksv1.PodGroupPolicySource{
					VolcanoScheduling: &arksv1.VolcanoSchedulingPodGroupPolicySource{
						PriorityClassName: "high-priority",
						Queue:             "default",
					},
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gang-volcano", f.Namespace).
				WithPodGroupPolicy(policy).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify RBGS has PodGroupPolicy
			f.ExpectRBGSHasPodGroupPolicy(app, "volcanoScheduling")

			// Verify complete gang scheduling chain: pods have Volcano PodGroup annotation
			f.ExpectVolcanoPodGroupAnnotation(app)
		})

		ginkgo.It("should propagate KubeScheduling scheduleTimeoutSeconds to RBGS", func() {
			f := *fp
			timeoutSeconds := int32(180)
			policy := &arksv1.PodGroupPolicy{
				PodGroupPolicySource: arksv1.PodGroupPolicySource{
					KubeScheduling: &arksv1.KubeSchedulingPodGroupPolicySource{
						ScheduleTimeoutSeconds: ptr.To(timeoutSeconds),
					},
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gang-timeout", f.Namespace).
				WithPodGroupPolicy(policy).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify the timeout is correctly propagated
			f.ExpectRBGSKubeSchedulingTimeout(app, timeoutSeconds)
		})

		ginkgo.It("should propagate VolcanoScheduling queue and priority to RBGS", func() {
			f := *fp
			policy := &arksv1.PodGroupPolicy{
				PodGroupPolicySource: arksv1.PodGroupPolicySource{
					VolcanoScheduling: &arksv1.VolcanoSchedulingPodGroupPolicySource{
						PriorityClassName: "system-cluster-critical",
						Queue:             "production",
					},
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-gang-volcano-config", f.Namespace).
				WithPodGroupPolicy(policy).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify volcano scheduling config is propagated
			f.ExpectRBGSVolcanoSchedulingConfig(app, "system-cluster-critical", "production")
		})

		ginkgo.It("should work with multi-replica gang scheduling", func() {
			f := *fp
			policy := &arksv1.PodGroupPolicy{
				PodGroupPolicySource: arksv1.PodGroupPolicySource{
					KubeScheduling: &arksv1.KubeSchedulingPodGroupPolicySource{
						ScheduleTimeoutSeconds: ptr.To(int32(120)),
					},
				},
			}

			// Create app with multiple replicas per role
			// Total pods = 2 (router) + 2*1 (prefill) + 2*1 (decode) = 6
			app := wrappers.BuildBasicArksDisaggApp("e2e-gang-multi", f.Namespace).
				WithPodGroupPolicy(policy).
				WithRouterReplicas(2).
				WithPrefillReplicas(2).
				WithPrefillSize(1).
				WithDecodeReplicas(2).
				WithDecodeSize(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify all replicas are ready (gang scheduling succeeded)
			f.ExpectRouterReplicas(app, 2)
			f.ExpectPrefillReplicas(app, 2)
			f.ExpectDecodeReplicas(app, 2)

			// Verify RBGS has PodGroupPolicy
			f.ExpectRBGSHasPodGroupPolicy(app, "kubeScheduling")
		})

		ginkgo.It("should work with large group size gang scheduling", func() {
			f := *fp
			policy := &arksv1.PodGroupPolicy{
				PodGroupPolicySource: arksv1.PodGroupPolicySource{
					KubeScheduling: &arksv1.KubeSchedulingPodGroupPolicySource{
						ScheduleTimeoutSeconds: ptr.To(int32(180)),
					},
				},
			}

			// Create app with larger group sizes (TP parallel)
			// Total pods = 1 (router) + 1*2 (prefill with size 2) + 1*2 (decode with size 2) = 5
			app := wrappers.BuildBasicArksDisaggApp("e2e-gang-large-group", f.Namespace).
				WithPodGroupPolicy(policy).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithPrefillSize(2).
				WithDecodeReplicas(1).
				WithDecodeSize(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify RBGS has PodGroupPolicy
			f.ExpectRBGSHasPodGroupPolicy(app, "kubeScheduling")
		})

		ginkgo.It("should create application without PodGroupPolicy (no gang scheduling)", func() {
			f := *fp
			// Create app without PodGroupPolicy - should work normally
			app := wrappers.BuildBasicArksDisaggApp("e2e-no-gang", f.Namespace).
				WithRouterReplicas(1).
				WithPrefillReplicas(1).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Verify RBGS exists but has no PodGroupPolicy
			f.ExpectRBGSNoPodGroupPolicy(app)
		})
	})
}
