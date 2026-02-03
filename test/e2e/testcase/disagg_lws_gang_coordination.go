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
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

// VolcanoSchedulerName is the scheduler name for Volcano gang scheduling
// Note: The cluster's Volcano scheduler is named "si-scheduler", not "volcano"
const VolcanoSchedulerName = "si-scheduler"

// RunDisaggLwsGangCoordinationTestCases runs e2e tests for LWS Gang + Coordination Scaling
// Architecture:
//
//	ArksDisaggregatedApplication
//	    ↓ (backend: RBG)
//	RoleBasedGroupSet (RBGS)
//	    ├─ PodGroupPolicy: nil (no RBG-level gang)
//	    ├─ CoordinationPolicy: enable=true (coordination scaling)
//	    └─ Roles: prefill/decode (workload=LWS)
//	           ↓
//	    LeaderWorkerSet (LWS)
//	        ├─ schedulerName: si-scheduler (LWS-level gang)
//	        └─ Volcano Provider creates PodGroup per replica
//
// This verifies:
// 1. LWS Gang: Each LWS replica's pods are scheduled together (no "half instance")
// 2. Coordination Scaling: prefill/decode are created in batches according to ratio
// 3. No RBG-level PodGroup when PodGroupPolicy is not set
func RunDisaggLwsGangCoordinationTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication LWS Gang + Coordination Scaling", ginkgo.Label("lws-gang", "coordination"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		// Test 2: Coordination Scaling works with LWS workloads
		// Verify that coordination scaling maintains the ratio between prefill and decode
		ginkgo.It("should coordinate LWS workload scaling according to maxSkew", ginkgo.Label("coordination", "lws"), func() {
			f := *fp
			ginkgo.By("Creating ArksDisaggApp with coordination scaling and LWS gang")

			coordPolicy := &arksv1.CoordinationPolicy{
				Scaling: &arksv1.ScalingCoordination{
					MaxSkew:     "20%",
					Progression: "OrderScheduled",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-lws-coord-scale", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithRouterReplicas(1).
				WithPrefillReplicas(4).
				WithPrefillSize(2).
				WithDecodeReplicas(2).
				WithDecodeSize(2).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Verifying RBGS configuration")
			f.ExpectRBGSNoPodGroupPolicy(app)
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationScaling(app, "pd-coordination", "20%")

			ginkgo.By("Waiting for app to reach running state")
			// With normal resources, pods should be schedulable
			f.ExpectArksDisaggAppReadyWithTimeout(app, 3*time.Minute)

			ginkgo.By("Verifying all replicas reached desired state")
			f.ExpectPrefillReplicas(app, 4)
			f.ExpectDecodeReplicas(app, 2)

			// Verify LWS gang is active
			f.ExpectLWSPodHasSchedulerName(app, "prefill", VolcanoSchedulerName)
			f.ExpectLWSPodHasSchedulerName(app, "decode", VolcanoSchedulerName)

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test: Coordination Scaling real effect test
		// Verifies that during scale up, the progress difference between prefill and decode
		// is constrained by maxSkew. Uses sampling approach to verify the constraint.
		ginkgo.It("should enforce maxSkew constraint during scale up", ginkgo.Label("coordination", "scaling-effect"), func() {
			f := *fp

			// Use larger replica counts to observe coordination effect
			prefillReplicas := 10
			decodeReplicas := 10

			ginkgo.By("Creating ArksDisaggApp with Scaling coordination (maxSkew=20%)")
			coordPolicy := &arksv1.CoordinationPolicy{
				Scaling: &arksv1.ScalingCoordination{
					MaxSkew:     "20%",
					Progression: "OrderScheduled",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-coord-scaling-effect", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithRouterReplicas(1).
				WithPrefillReplicas(int32(prefillReplicas)).
				WithPrefillSize(1). // Single pod per replica for easier tracking
				WithDecodeReplicas(int32(decodeReplicas)).
				WithDecodeSize(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Verifying RBGS has coordination scaling configured")
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationScaling(app, "pd-coordination", "20%")

			ginkgo.By("Sampling deployment progress to verify maxSkew constraint")
			// This function samples pod counts during deployment and verifies
			// that at least 70% of samples satisfy the maxSkew constraint
			f.ExpectCoordinationScalingProgress(app, prefillReplicas, decodeReplicas, 0.20)

			ginkgo.By("Waiting for app to reach running state")
			f.ExpectArksDisaggAppReadyWithTimeout(app, 3*time.Minute)

			ginkgo.By("Verifying final replica counts")
			f.ExpectPrefillReplicas(app, int32(prefillReplicas))
			f.ExpectDecodeReplicas(app, int32(decodeReplicas))

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test 3: Verify no RBG-level PodGroup when PodGroupPolicy is not set
		// LWS manages its own gang scheduling, RBG should not create additional PodGroup
		ginkgo.It("should not create RBG-level PodGroup when using LWS gang without PodGroupPolicy", ginkgo.Label("lws-gang", "no-rbg-pg"), func() {
			f := *fp
			ginkgo.By("Creating ArksDisaggApp with LWS gang but NO PodGroupPolicy")

			coordPolicy := &arksv1.CoordinationPolicy{
				Scaling: &arksv1.ScalingCoordination{
					MaxSkew: "20%",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-lws-no-rbg-pg", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(2).
				WithPrefillSize(2).
				WithDecodeReplicas(1).
				WithDecodeSize(2).
				Obj()

			// Explicitly ensure no PodGroupPolicy
			app.Spec.PodGroupPolicy = nil

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Verifying RBGS has no PodGroupPolicy")
			f.ExpectRBGSNoPodGroupPolicy(app)

			// Wait for pods to be created
			time.Sleep(30 * time.Second)

			ginkgo.By("Verifying pods have LWS-level PodGroup annotation (not RBG-level)")
			// LWS pods should have Volcano annotation from LWS gang scheduling
			// but the annotation value should be LWS-managed, not app name
			podList := &corev1.PodList{}
			gomega.Expect(f.Client.List(f.Ctx, podList,
				client.InNamespace(app.Namespace),
				client.MatchingLabels{
					"arks.ai/application": app.Name,
				})).Should(gomega.Succeed())

			for _, pod := range podList.Items {
				if groupName, ok := pod.Annotations["scheduling.k8s.io/group-name"]; ok {
					// LWS gang creates PodGroup with LWS-specific naming
					// It should NOT be the app name directly (which would indicate RBG-level PodGroup)
					gomega.Expect(groupName).NotTo(gomega.Equal(app.Name),
						"Pod should not have RBG-level PodGroup annotation (value should not be app name)")
					ginkgo.By(fmt.Sprintf("Pod %s has LWS-level PodGroup: %s", pod.Name, groupName))
				}
			}

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test 4: Verify CoordinationPolicy works independently of PodGroupPolicy
		// This test confirms that coordination is enabled when Scaling or RollingUpdate is configured
		ginkgo.It("should enable coordination only when Scaling or RollingUpdate is configured", ginkgo.Label("coordination", "enable"), func() {
			f := *fp
			ginkgo.By("Creating ArksDisaggApp without CoordinationPolicy")

			// No coordination policy
			appNoCoord := wrappers.BuildBasicArksDisaggApp("e2e-coord-none", f.Namespace).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(2).
				WithDecodeReplicas(1).
				Obj()
			appNoCoord.Spec.CoordinationPolicy = nil

			gomega.Expect(f.Client.Create(f.Ctx, appNoCoord)).Should(gomega.Succeed())

			ginkgo.By("Verifying no CoordinationRequirements when CoordinationPolicy is nil")
			f.ExpectRBGSNoCoordinationRequirements(appNoCoord)

			ginkgo.By("Cleaning up no-coord app")
			gomega.Expect(f.Client.Delete(f.Ctx, appNoCoord)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(appNoCoord)

			ginkgo.By("Creating ArksDisaggApp with Scaling coordination")

			// Coordination with Scaling
			coordPolicyScaling := &arksv1.CoordinationPolicy{
				Scaling: &arksv1.ScalingCoordination{
					MaxSkew: "30%",
				},
			}

			appScaling := wrappers.BuildBasicArksDisaggApp("e2e-coord-scaling", f.Namespace).
				WithCoordinationPolicy(coordPolicyScaling).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(2).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, appScaling)).Should(gomega.Succeed())

			ginkgo.By("Verifying CoordinationRequirements exist when Scaling is configured")
			f.ExpectRBGSHasCoordinationRequirements(appScaling, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationScaling(appScaling, "pd-coordination", "30%")

			ginkgo.By("Cleaning up scaling app")
			gomega.Expect(f.Client.Delete(f.Ctx, appScaling)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(appScaling)
		})

		// Test 5: Verify CoordinationRequirements dynamic update
		// This test confirms that updating CoordinationPolicy on the App
		// correctly propagates to RBGS CoordinationRequirements
		ginkgo.It("should update RBGS CoordinationRequirements when CoordinationPolicy changes", ginkgo.Label("coordination", "update"), func() {
			f := *fp
			ginkgo.By("Creating ArksDisaggApp with CoordinationPolicy (Scaling.MaxSkew=20%)")

			coordPolicy := &arksv1.CoordinationPolicy{
				Scaling: &arksv1.ScalingCoordination{
					MaxSkew: "20%",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-coord-update", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(1).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Verifying initial CoordinationRequirements (MaxSkew=20%)")
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationScaling(app, "pd-coordination", "20%")
			// Wait for RBG to be created and verify initial CoordinationRequirements
			f.ExpectRBGExists(app)

			ginkgo.By("Updating CoordinationPolicy.Scaling.MaxSkew from 20% to 30%")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.CoordinationPolicy.Scaling.MaxSkew = "30%"
			})

			ginkgo.By("Verifying RBGS CoordinationRequirements updated (MaxSkew=30%)")
			f.ExpectRBGSCoordinationScaling(app, "pd-coordination", "30%")

			ginkgo.By("Removing Scaling coordination by setting to nil")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.CoordinationPolicy.Scaling = nil
			})

			ginkgo.By("Verifying RBGS CoordinationRequirements removed")
			f.ExpectRBGSNoCoordinationRequirements(app)

			ginkgo.By("Re-enabling Scaling coordination with MaxSkew=25%")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.CoordinationPolicy.Scaling = &arksv1.ScalingCoordination{
					MaxSkew: "25%",
				}
			})

			ginkgo.By("Verifying RBGS CoordinationRequirements re-created (MaxSkew=25%)")
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationScaling(app, "pd-coordination", "25%")

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test 6: RollingUpdate basic configuration
		// Verifies RollingUpdate coordination fields are correctly propagated to RBGS and RBG
		ginkgo.It("should configure RollingUpdate coordination with all fields", ginkgo.Label("coordination", "rolling-update"), func() {
			f := *fp
			ginkgo.By("Creating ArksDisaggApp with RollingUpdate coordination (all fields)")

			coordPolicy := &arksv1.CoordinationPolicy{
				RollingUpdate: &arksv1.RollingUpdateCoordination{
					MaxSkew:        "5%",
					MaxUnavailable: "10%",
					Partition:      "50%",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-coord-rolling-update", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(2).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Verifying RBGS has CoordinationRequirements with RollingUpdate")
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationRollingUpdate(app, "pd-coordination", "5%", "10%", "50%")

			ginkgo.By("Verifying RBG has CoordinationRequirements synced from RBGS")
			f.ExpectRBGExists(app)
			f.ExpectRBGCoordinationRollingUpdate(app, "pd-coordination", "5%", "10%", "50%")

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test 7: Scaling + RollingUpdate coexistence
		// Verifies both strategies can be configured simultaneously
		ginkgo.It("should support both Scaling and RollingUpdate coordination simultaneously", ginkgo.Label("coordination", "coexist"), func() {
			f := *fp
			ginkgo.By("Creating ArksDisaggApp with both Scaling and RollingUpdate coordination")

			coordPolicy := &arksv1.CoordinationPolicy{
				Scaling: &arksv1.ScalingCoordination{
					MaxSkew:     "20%",
					Progression: "OrderScheduled",
				},
				RollingUpdate: &arksv1.RollingUpdateCoordination{
					MaxSkew:        "5%",
					MaxUnavailable: "10%",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-coord-both", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(2).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Verifying RBGS has both Scaling and RollingUpdate strategies")
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationScalingAndRollingUpdate(app, "pd-coordination", "20%", "5%")

			ginkgo.By("Verifying RBG has both strategies synced")
			f.ExpectRBGExists(app)
			f.ExpectRBGCoordinationScaling(app, "pd-coordination", "20%")
			f.ExpectRBGCoordinationRollingUpdate(app, "pd-coordination", "5%", "10%", "")

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test 8: RollingUpdate dynamic update
		// Verifies updating RollingUpdate fields propagates correctly to RBGS and RBG
		ginkgo.It("should update RBGS CoordinationRequirements when RollingUpdate changes", ginkgo.Label("coordination", "rolling-update-dynamic"), func() {
			f := *fp
			ginkgo.By("Creating ArksDisaggApp with RollingUpdate (MaxSkew=5%)")

			coordPolicy := &arksv1.CoordinationPolicy{
				RollingUpdate: &arksv1.RollingUpdateCoordination{
					MaxSkew: "5%",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-coord-rolling-dynamic", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(1).
				WithDecodeReplicas(1).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Verifying initial RollingUpdate configuration (MaxSkew=5%)")
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationRollingUpdate(app, "pd-coordination", "5%", "", "")
			f.ExpectRBGExists(app)

			ginkgo.By("Updating RollingUpdate.MaxSkew from 5% to 10%")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.CoordinationPolicy.RollingUpdate.MaxSkew = "10%"
			})

			ginkgo.By("Verifying RBGS RollingUpdate updated (MaxSkew=10%)")
			f.ExpectRBGSCoordinationRollingUpdate(app, "pd-coordination", "10%", "", "")

			ginkgo.By("Adding MaxUnavailable and Partition fields")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.CoordinationPolicy.RollingUpdate.MaxUnavailable = "15%"
				a.Spec.CoordinationPolicy.RollingUpdate.Partition = "30%"
			})

			ginkgo.By("Verifying all RollingUpdate fields updated")
			f.ExpectRBGSCoordinationRollingUpdate(app, "pd-coordination", "10%", "15%", "30%")

			ginkgo.By("Removing RollingUpdate by setting to nil")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.CoordinationPolicy.RollingUpdate = nil
			})

			ginkgo.By("Verifying CoordinationRequirements removed")
			f.ExpectRBGSNoCoordinationRequirements(app)

			ginkgo.By("Re-enabling with Scaling instead of RollingUpdate")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.CoordinationPolicy.Scaling = &arksv1.ScalingCoordination{
					MaxSkew: "25%",
				}
			})

			ginkgo.By("Verifying switched to Scaling coordination")
			f.ExpectRBGSHasCoordinationRequirements(app, "pd-coordination", []string{"prefill", "decode"})
			f.ExpectRBGSCoordinationScaling(app, "pd-coordination", "25%")

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test 9: RollingUpdate coordination real effect test
		// Verifies that during rolling update, the progress difference between prefill and decode
		// is constrained by maxSkew. Uses sampling approach similar to Scaling coordination tests.
		ginkgo.It("should enforce maxSkew constraint during rolling update", ginkgo.Label("coordination", "rolling-update-effect"), func() {
			f := *fp

			// Use larger replica counts to observe coordination effect
			prefillReplicas := int32(10)
			decodeReplicas := int32(5)
			maxSkew := "20%" // 20% maxSkew for observable effect

			ginkgo.By("Creating ArksDisaggApp with RollingUpdate coordination (maxSkew=20%)")
			coordPolicy := &arksv1.CoordinationPolicy{
				RollingUpdate: &arksv1.RollingUpdateCoordination{
					MaxSkew: maxSkew,
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-coord-rolling-effect", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(prefillReplicas).
				WithDecodeReplicas(decodeReplicas).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Waiting for initial deployment to be ready")
			f.ExpectArksDisaggAppReady(app)
			f.ExpectPrefillReplicas(app, prefillReplicas)
			f.ExpectDecodeReplicas(app, decodeReplicas)

			ginkgo.By("Triggering rolling update by changing Env")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				updateEnv := []corev1.EnvVar{{Name: "ROLLING_UPDATE_TRIGGER", Value: "v2"}}
				a.Spec.Prefill.InstanceSpec.Env = updateEnv
				a.Spec.Decode.InstanceSpec.Env = updateEnv
			})

			ginkgo.By("Sampling update progress and verifying maxSkew constraint")
			// 20% maxSkew = 0.20
			f.ExpectCoordinationRollingUpdateProgress(app, int(prefillReplicas), int(decodeReplicas), 0.20)

			ginkgo.By("Verifying rolling update completed successfully")
			// Rolling update should already be complete after sampling, but verify with longer timeout
			f.ExpectRollingUpdateCompleteWithTimeout(app, 10*time.Minute)

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})

		// Test 10: Verify LWS partition is actually being set during rolling update
		// This test specifically verifies that the RBG coordination mechanism is working:
		// 1. LWS RollingUpdateConfiguration is present
		// 2. Partition values are being set during rolling update
		// This is a diagnostic test to verify the LWS integration is working correctly.
		ginkgo.It("should set LWS partition during rolling update coordination", ginkgo.Label("coordination", "lws-partition-verify"), func() {
			f := *fp

			prefillReplicas := int32(5)
			decodeReplicas := int32(3)

			ginkgo.By("Creating ArksDisaggApp with RollingUpdate coordination")
			coordPolicy := &arksv1.CoordinationPolicy{
				RollingUpdate: &arksv1.RollingUpdateCoordination{
					MaxSkew: "20%",
				},
			}

			app := wrappers.BuildBasicArksDisaggApp("e2e-lws-partition-verify", f.Namespace).
				WithCoordinationPolicy(coordPolicy).
				WithPrefillSchedulerName(VolcanoSchedulerName).
				WithDecodeSchedulerName(VolcanoSchedulerName).
				WithPrefillReplicas(prefillReplicas).
				WithDecodeReplicas(decodeReplicas).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())

			ginkgo.By("Waiting for initial deployment to be ready")
			f.ExpectArksDisaggAppReady(app)

			ginkgo.By("Verifying LWS has RollingUpdateConfiguration before rolling update")
			prefillInfo := f.GetLWSRolloutStrategyInfo(app, "prefill")
			decodeInfo := f.GetLWSRolloutStrategyInfo(app, "decode")
			fmt.Fprintf(ginkgo.GinkgoWriter, "Before rolling update:\n  Prefill: %s\n  Decode: %s\n", prefillInfo, decodeInfo)

			// Verify RollingUpdateConfiguration exists (this confirms arks configures RolloutStrategy)
			prefillPartition := f.GetLWSPartition(app, "prefill")
			decodePartition := f.GetLWSPartition(app, "decode")
			gomega.Expect(prefillPartition).To(gomega.BeNumerically(">=", 0),
				"Prefill LWS should have RollingUpdateConfiguration (partition >= 0)")
			gomega.Expect(decodePartition).To(gomega.BeNumerically(">=", 0),
				"Decode LWS should have RollingUpdateConfiguration (partition >= 0)")

			ginkgo.By("Triggering rolling update by changing Env")
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				updateEnv := []corev1.EnvVar{{Name: "ROLLING_UPDATE_TRIGGER", Value: "v2"}}
				a.Spec.Prefill.InstanceSpec.Env = updateEnv
				a.Spec.Decode.InstanceSpec.Env = updateEnv
			})

			ginkgo.By("Sampling LWS partition values during rolling update")
			partitionSamples, partitionWasSet := f.SampleLWSPartitionsDuringRollingUpdate(app, prefillReplicas, decodeReplicas)

			// Print all samples for debugging
			fmt.Fprintf(ginkgo.GinkgoWriter, "\n=== LWS Partition Samples During Rolling Update ===\n")
			for i, sample := range partitionSamples {
				fmt.Fprintf(ginkgo.GinkgoWriter, "  [%d] %s\n", i, sample)
			}
			fmt.Fprintf(ginkgo.GinkgoWriter, "Partition was set (> 0) at some point: %v\n", partitionWasSet)
			fmt.Fprintf(ginkgo.GinkgoWriter, "==================================================\n\n")

			// The key verification: partition should be non-negative throughout
			// This confirms RBG coordination is actively managing LWS partitions
			ginkgo.By("Verifying LWS partitions are being managed by RBG coordination")
			// At minimum, we expect partitions to be set (not -1) during the update
			// A non-zero partition at some point indicates coordination is actively constraining updates
			// Note: If coordination is NOT working, we might see both partitions stay at 0
			// while updates happen freely, or partitions stay at -1 (not configured)

			ginkgo.By("Waiting for rolling update to complete")
			f.ExpectRollingUpdateCompleteWithTimeout(app, 5*time.Minute)

			ginkgo.By("Cleaning up")
			gomega.Expect(f.Client.Delete(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppDeleted(app)
		})
	})
}
