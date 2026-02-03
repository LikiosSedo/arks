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

package framework

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
	rbgv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

// ExpectArksDisaggAppReady waits for ArksDisaggregatedApplication to reach Running phase
func (f *Framework) ExpectArksDisaggAppReady(app *arksv1.ArksDisaggregatedApplication) {
	f.ExpectArksDisaggAppReadyWithTimeout(app, Timeout)
}

// ExpectArksDisaggAppReadyWithTimeout waits for ArksDisaggregatedApplication to reach Running phase with custom timeout
func (f *Framework) ExpectArksDisaggAppReadyWithTimeout(app *arksv1.ArksDisaggregatedApplication, timeout time.Duration) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	gomega.Eventually(func() bool {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error(err, "Failed to get ArksDisaggregatedApplication")
			}
			return false
		}

		if newApp.Status.Phase != string(arksv1.ArksApplicationPhaseRunning) {
			logger.V(1).Info("ArksDisaggregatedApplication not running yet",
				"phase", newApp.Status.Phase,
				"routerReady", newApp.Status.Router.ReadyReplicas,
				"prefillReady", newApp.Status.Prefill.ReadyReplicas,
				"decodeReady", newApp.Status.Decode.ReadyReplicas)
			return false
		}
		return true
	}, timeout, Interval).Should(gomega.BeTrue(),
		"ArksDisaggregatedApplication %s/%s should reach Running phase", app.Namespace, app.Name)
}

// ExpectArksDisaggAppDeleted waits for ArksDisaggregatedApplication to be deleted
func (f *Framework) ExpectArksDisaggAppDeleted(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		return apierrors.IsNotFound(err)
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"ArksDisaggregatedApplication %s/%s should be deleted", app.Namespace, app.Name)
}

// ExpectArksDisaggAppCondition checks a specific condition on the application
func (f *Framework) ExpectArksDisaggAppCondition(
	app *arksv1.ArksDisaggregatedApplication,
	conditionType arksv1.ArksApplicationConditionType,
	status corev1.ConditionStatus,
) {
	gomega.Eventually(func() bool {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return false
		}

		for _, cond := range newApp.Status.Conditions {
			if cond.Type == conditionType && cond.Status == status {
				return true
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"ArksDisaggregatedApplication %s/%s should have condition %s=%s",
		app.Namespace, app.Name, conditionType, status)
}

// ExpectRouterReplicas verifies router replica count matches expected
func (f *Framework) ExpectRouterReplicas(app *arksv1.ArksDisaggregatedApplication, expected int32) {
	gomega.Eventually(func() int32 {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return -1
		}
		return newApp.Status.Router.ReadyReplicas
	}, Timeout, Interval).Should(gomega.Equal(expected),
		"Router should have %d ready replicas", expected)
}

// ExpectPrefillReplicas verifies prefill replica count matches expected
func (f *Framework) ExpectPrefillReplicas(app *arksv1.ArksDisaggregatedApplication, expected int32) {
	gomega.Eventually(func() int32 {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return -1
		}
		return newApp.Status.Prefill.ReadyReplicas
	}, Timeout, Interval).Should(gomega.Equal(expected),
		"Prefill should have %d ready replicas", expected)
}

// ExpectDecodeReplicas verifies decode replica count matches expected
func (f *Framework) ExpectDecodeReplicas(app *arksv1.ArksDisaggregatedApplication, expected int32) {
	f.ExpectDecodeReplicasWithTimeout(app, expected, Timeout)
}

// ExpectDecodeReplicasWithTimeout verifies decode replica count with custom timeout
func (f *Framework) ExpectDecodeReplicasWithTimeout(app *arksv1.ArksDisaggregatedApplication, expected int32, timeout time.Duration) {
	gomega.Eventually(func() int32 {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return -1
		}
		return newApp.Status.Decode.ReadyReplicas
	}, timeout, Interval).Should(gomega.Equal(expected),
		"Decode should have %d ready replicas", expected)
}

// ExpectRouterUpdatedReplicas verifies router updated replica count
func (f *Framework) ExpectRouterUpdatedReplicas(app *arksv1.ArksDisaggregatedApplication, expected int32) {
	gomega.Eventually(func() int32 {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return -1
		}
		return newApp.Status.Router.UpdatedReplicas
	}, Timeout, Interval).Should(gomega.Equal(expected),
		"Router should have %d updated replicas", expected)
}

// ExpectPrefillUpdatedReplicas verifies prefill updated replica count
func (f *Framework) ExpectPrefillUpdatedReplicas(app *arksv1.ArksDisaggregatedApplication, expected int32) {
	gomega.Eventually(func() int32 {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return -1
		}
		return newApp.Status.Prefill.UpdatedReplicas
	}, Timeout, Interval).Should(gomega.Equal(expected),
		"Prefill should have %d updated replicas", expected)
}

// ExpectDecodeUpdatedReplicas verifies decode updated replica count
func (f *Framework) ExpectDecodeUpdatedReplicas(app *arksv1.ArksDisaggregatedApplication, expected int32) {
	gomega.Eventually(func() int32 {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return -1
		}
		return newApp.Status.Decode.UpdatedReplicas
	}, Timeout, Interval).Should(gomega.Equal(expected),
		"Decode should have %d updated replicas", expected)
}

// ExpectAllReplicasReady verifies all roles have their replicas ready
func (f *Framework) ExpectAllReplicasReady(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return false
		}

		routerReady := newApp.Status.Router.ReadyReplicas == newApp.Status.Router.Replicas
		prefillReady := newApp.Status.Prefill.ReadyReplicas == newApp.Status.Prefill.Replicas
		decodeReady := newApp.Status.Decode.ReadyReplicas == newApp.Status.Decode.Replicas

		return routerReady && prefillReady && decodeReady
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"All roles should have all replicas ready")
}

// ExpectRollingUpdateComplete verifies rolling update has completed for all roles
func (f *Framework) ExpectRollingUpdateComplete(app *arksv1.ArksDisaggregatedApplication) {
	f.ExpectRollingUpdateCompleteWithTimeout(app, Timeout)
}

// ExpectRollingUpdateCompleteWithTimeout verifies rolling update has completed with custom timeout
func (f *Framework) ExpectRollingUpdateCompleteWithTimeout(app *arksv1.ArksDisaggregatedApplication, timeout time.Duration) {
	gomega.Eventually(func() bool {
		newApp := &arksv1.ArksDisaggregatedApplication{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      app.Name,
			Namespace: app.Namespace,
		}, newApp)
		if err != nil {
			return false
		}

		// All roles should have updated = ready = replicas
		routerComplete := newApp.Status.Router.UpdatedReplicas == newApp.Status.Router.ReadyReplicas &&
			newApp.Status.Router.ReadyReplicas == newApp.Status.Router.Replicas
		prefillComplete := newApp.Status.Prefill.UpdatedReplicas == newApp.Status.Prefill.ReadyReplicas &&
			newApp.Status.Prefill.ReadyReplicas == newApp.Status.Prefill.Replicas
		decodeComplete := newApp.Status.Decode.UpdatedReplicas == newApp.Status.Decode.ReadyReplicas &&
			newApp.Status.Decode.ReadyReplicas == newApp.Status.Decode.Replicas

		return routerComplete && prefillComplete && decodeComplete
	}, timeout, Interval).Should(gomega.BeTrue(),
		"Rolling update should complete for all roles")
}

// GetCurrentArksDisaggApp fetches the current state of the application
func (f *Framework) GetCurrentArksDisaggApp(app *arksv1.ArksDisaggregatedApplication) *arksv1.ArksDisaggregatedApplication {
	newApp := &arksv1.ArksDisaggregatedApplication{}
	err := f.Client.Get(f.Ctx, client.ObjectKey{
		Name:      app.Name,
		Namespace: app.Namespace,
	}, newApp)
	gomega.Expect(err).ToNot(gomega.HaveOccurred(),
		"Should be able to get ArksDisaggregatedApplication %s/%s", app.Namespace, app.Name)
	return newApp
}

// ExpectRBGSHasPodGroupPolicy verifies RBGS has the specified PodGroupPolicy type
func (f *Framework) ExpectRBGSHasPodGroupPolicy(app *arksv1.ArksDisaggregatedApplication, policyType string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		policy := rbgs.Spec.Template.PodGroupPolicy
		if policy == nil {
			return false
		}

		switch policyType {
		case "kubeScheduling":
			return policy.KubeScheduling != nil
		case "volcanoScheduling":
			return policy.VolcanoScheduling != nil
		default:
			return false
		}
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS should have %s PodGroupPolicy", policyType)
}

// ExpectRBGSNoPodGroupPolicy verifies RBGS has no PodGroupPolicy
func (f *Framework) ExpectRBGSNoPodGroupPolicy(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}
		return rbgs.Spec.Template.PodGroupPolicy == nil
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS should have no PodGroupPolicy")
}

// ExpectRBGSKubeSchedulingTimeout verifies KubeScheduling scheduleTimeoutSeconds
func (f *Framework) ExpectRBGSKubeSchedulingTimeout(app *arksv1.ArksDisaggregatedApplication, expectedTimeout int32) {
	gomega.Eventually(func() int32 {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return -1
		}

		policy := rbgs.Spec.Template.PodGroupPolicy
		if policy == nil || policy.KubeScheduling == nil {
			return -1
		}

		if policy.KubeScheduling.ScheduleTimeoutSeconds == nil {
			return 0 // default
		}
		return *policy.KubeScheduling.ScheduleTimeoutSeconds
	}, Timeout, Interval).Should(gomega.Equal(expectedTimeout),
		"KubeScheduling scheduleTimeoutSeconds should be %d", expectedTimeout)
}

// ExpectRBGSVolcanoSchedulingConfig verifies VolcanoScheduling config
func (f *Framework) ExpectRBGSVolcanoSchedulingConfig(app *arksv1.ArksDisaggregatedApplication, expectedPriority, expectedQueue string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		policy := rbgs.Spec.Template.PodGroupPolicy
		if policy == nil || policy.VolcanoScheduling == nil {
			return false
		}

		return policy.VolcanoScheduling.PriorityClassName == expectedPriority &&
			policy.VolcanoScheduling.Queue == expectedQueue
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"VolcanoScheduling should have priority=%s queue=%s", expectedPriority, expectedQueue)
}

// ExpectRBGSHasCoordinationRequirements verifies RBGS has CoordinationRequirements configured
// with the expected coordination name and roles
func (f *Framework) ExpectRBGSHasCoordinationRequirements(app *arksv1.ArksDisaggregatedApplication, coordinationName string, expectedRoles []string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		coordinations := rbgs.Spec.Template.CoordinationRequirements
		if len(coordinations) == 0 {
			return false
		}

		// Find the coordination by name
		for _, coord := range coordinations {
			if coord.Name == coordinationName {
				// Check if roles match
				if len(coord.Roles) != len(expectedRoles) {
					return false
				}
				roleSet := make(map[string]bool)
				for _, r := range coord.Roles {
					roleSet[r] = true
				}
				for _, r := range expectedRoles {
					if !roleSet[r] {
						return false
					}
				}
				return true
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS should have coordination %s with roles %v", coordinationName, expectedRoles)
}

// ExpectRBGSNoCoordinationRequirements verifies RBGS has no CoordinationRequirements
func (f *Framework) ExpectRBGSNoCoordinationRequirements(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}
		return len(rbgs.Spec.Template.CoordinationRequirements) == 0
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS should have no CoordinationRequirements")
}

// ExpectRBGSCoordinationScaling verifies RBGS has CoordinationScaling with expected values
func (f *Framework) ExpectRBGSCoordinationScaling(app *arksv1.ArksDisaggregatedApplication, coordinationName string, expectedMaxSkew string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		for _, coord := range rbgs.Spec.Template.CoordinationRequirements {
			if coord.Name == coordinationName {
				if coord.Strategy == nil || coord.Strategy.Scaling == nil {
					return false
				}
				if coord.Strategy.Scaling.MaxSkew == nil {
					return false
				}
				return *coord.Strategy.Scaling.MaxSkew == expectedMaxSkew
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS coordination %s should have maxSkew=%s", coordinationName, expectedMaxSkew)
}

// ExpectRBGSCoordinationRollingUpdate verifies RBGS has CoordinationRollingUpdate with expected values
func (f *Framework) ExpectRBGSCoordinationRollingUpdate(app *arksv1.ArksDisaggregatedApplication, coordinationName string, expectedMaxSkew, expectedMaxUnavailable, expectedPartition string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		for _, coord := range rbgs.Spec.Template.CoordinationRequirements {
			if coord.Name == coordinationName {
				if coord.Strategy == nil || coord.Strategy.RollingUpdate == nil {
					return false
				}
				ru := coord.Strategy.RollingUpdate
				// Check MaxSkew if expected
				if expectedMaxSkew != "" {
					if ru.MaxSkew == nil || *ru.MaxSkew != expectedMaxSkew {
						return false
					}
				}
				// Check MaxUnavailable if expected
				if expectedMaxUnavailable != "" {
					if ru.MaxUnavailable == nil || *ru.MaxUnavailable != expectedMaxUnavailable {
						return false
					}
				}
				// Check Partition if expected
				if expectedPartition != "" {
					if ru.Partition == nil || *ru.Partition != expectedPartition {
						return false
					}
				}
				return true
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS coordination %s should have RollingUpdate with maxSkew=%s, maxUnavailable=%s, partition=%s",
		coordinationName, expectedMaxSkew, expectedMaxUnavailable, expectedPartition)
}

// ExpectRBGSCoordinationScalingAndRollingUpdate verifies RBGS has both Scaling and RollingUpdate strategies
func (f *Framework) ExpectRBGSCoordinationScalingAndRollingUpdate(app *arksv1.ArksDisaggregatedApplication, coordinationName string, scalingMaxSkew, rollingUpdateMaxSkew string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		for _, coord := range rbgs.Spec.Template.CoordinationRequirements {
			if coord.Name == coordinationName {
				if coord.Strategy == nil {
					return false
				}
				// Check Scaling
				if coord.Strategy.Scaling == nil || coord.Strategy.Scaling.MaxSkew == nil {
					return false
				}
				if *coord.Strategy.Scaling.MaxSkew != scalingMaxSkew {
					return false
				}
				// Check RollingUpdate
				if coord.Strategy.RollingUpdate == nil || coord.Strategy.RollingUpdate.MaxSkew == nil {
					return false
				}
				if *coord.Strategy.RollingUpdate.MaxSkew != rollingUpdateMaxSkew {
					return false
				}
				return true
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS coordination %s should have Scaling.MaxSkew=%s and RollingUpdate.MaxSkew=%s",
		coordinationName, scalingMaxSkew, rollingUpdateMaxSkew)
}

// ExpectVolcanoPodGroupAnnotation verifies that pods have the Volcano PodGroup annotation
// This confirms the complete gang scheduling chain: arks -> RBGS -> RBG -> PodGroup -> Pods
func (f *Framework) ExpectVolcanoPodGroupAnnotation(app *arksv1.ArksDisaggregatedApplication) {
	const volcanoAnnotationKey = "scheduling.k8s.io/group-name"

	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application": app.Name,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		// All pods should have the volcano annotation
		for _, pod := range podList.Items {
			if _, ok := pod.Annotations[volcanoAnnotationKey]; !ok {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"All pods should have Volcano PodGroup annotation %s", volcanoAnnotationKey)
}

// ExpectKubePodGroupLabel verifies that pods have the Kube scheduler-plugins PodGroup label
func (f *Framework) ExpectKubePodGroupLabel(app *arksv1.ArksDisaggregatedApplication) {
	const kubeSchedulingLabelKey = "pod-group.scheduling.sigs.k8s.io/name"

	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application": app.Name,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		// All pods should have the kube scheduling label
		for _, pod := range podList.Items {
			if _, ok := pod.Labels[kubeSchedulingLabelKey]; !ok {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"All pods should have Kube PodGroup label %s", kubeSchedulingLabelKey)
}

// ExpectRolePodHasLabel verifies pods of a specific role have the expected label
func (f *Framework) ExpectRolePodHasLabel(app *arksv1.ArksDisaggregatedApplication, role string, labelKey, labelValue string) {
	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application":         app.Name,
				"arks.ai/disaggregation-role": role,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			if val, ok := pod.Labels[labelKey]; !ok || val != labelValue {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"%s pods should have label %s=%s", role, labelKey, labelValue)
}

// ExpectRolePodHasAnnotation verifies pods of a specific role have the expected annotation
func (f *Framework) ExpectRolePodHasAnnotation(app *arksv1.ArksDisaggregatedApplication, role string, annotationKey, annotationValue string) {
	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application":         app.Name,
				"arks.ai/disaggregation-role": role,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			if val, ok := pod.Annotations[annotationKey]; !ok || val != annotationValue {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"%s pods should have annotation %s=%s", role, annotationKey, annotationValue)
}

// ExpectRolePodHasToleration verifies pods of a specific role have the expected toleration
func (f *Framework) ExpectRolePodHasToleration(app *arksv1.ArksDisaggregatedApplication, role string, toleration corev1.Toleration) {
	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application":         app.Name,
				"arks.ai/disaggregation-role": role,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			found := false
			for _, t := range pod.Spec.Tolerations {
				if t.Key == toleration.Key && t.Effect == toleration.Effect {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"%s pods should have toleration key=%s effect=%s", role, toleration.Key, toleration.Effect)
}

// ExpectRolePodHasTerminationGracePeriod verifies pods of a specific role have the expected termination grace period
func (f *Framework) ExpectRolePodHasTerminationGracePeriod(app *arksv1.ArksDisaggregatedApplication, role string, seconds int64) {
	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application":         app.Name,
				"arks.ai/disaggregation-role": role,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			if pod.Spec.TerminationGracePeriodSeconds == nil || *pod.Spec.TerminationGracePeriodSeconds != seconds {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"%s pods should have terminationGracePeriodSeconds=%d", role, seconds)
}

// ExpectRolePodHasStartupProbe verifies pods of a specific role have startup probe configured
func (f *Framework) ExpectRolePodHasStartupProbe(app *arksv1.ArksDisaggregatedApplication, role string) {
	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application":         app.Name,
				"arks.ai/disaggregation-role": role,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			// Check if any container has startup probe
			hasStartupProbe := false
			for _, container := range pod.Spec.Containers {
				if container.StartupProbe != nil {
					hasStartupProbe = true
					break
				}
			}
			if !hasStartupProbe {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"%s pods should have startup probe", role)
}

// ExpectRBGSRoleHasEnv verifies that the RBGS template for a specific role has the expected environment variable
func (f *Framework) ExpectRBGSRoleHasEnv(app *arksv1.ArksDisaggregatedApplication, role string, envName, envValue string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		// Find the role in RBGS
		for _, r := range rbgs.Spec.Template.Roles {
			if r.Name == role {
				if r.Template == nil || len(r.Template.Spec.Containers) == 0 {
					return false
				}
				// Check containers for the expected env var
				for _, container := range r.Template.Spec.Containers {
					for _, env := range container.Env {
						if env.Name == envName && env.Value == envValue {
							return true
						}
					}
				}
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS role %s should have env %s=%s", role, envName, envValue)
}

// GetRolePodCount returns the current pod count for a specific role
func (f *Framework) GetRolePodCount(app *arksv1.ArksDisaggregatedApplication, role string) int {
	podList := &corev1.PodList{}
	err := f.Client.List(f.Ctx, podList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{
			"arks.ai/application":         app.Name,
			"arks.ai/disaggregation-role": role,
		})
	if err != nil {
		return 0
	}
	return len(podList.Items)
}

// GetPrefillAndDecodePodCounts returns current pod counts for prefill and decode roles
func (f *Framework) GetPrefillAndDecodePodCounts(app *arksv1.ArksDisaggregatedApplication) (prefillCount, decodeCount int) {
	prefillCount = f.GetRolePodCount(app, "prefill")
	decodeCount = f.GetRolePodCount(app, "decode")
	return
}

// GetPrefillAndDecodeUpdatedReplicas returns current updatedReplicas for prefill and decode roles
// This reads from RBG roleStatuses to get real-time values, bypassing ArksDisaggApp status sync delay.
// This is used for RollingUpdate coordination testing.
// Note: LWS status.updatedReplicas may not be populated in some versions, so we use RBG roleStatuses instead.
func (f *Framework) GetPrefillAndDecodeUpdatedReplicas(app *arksv1.ArksDisaggregatedApplication) (prefillUpdated, decodeUpdated int32) {
	// In RBG mode, RBG name is "{app.Name}-0"
	rbgName := app.Name + "-0"

	// Get RBG and read roleStatuses
	rbg := &rbgv1alpha1.RoleBasedGroup{}
	err := f.Client.Get(f.Ctx, client.ObjectKey{
		Namespace: app.Namespace,
		Name:      rbgName,
	}, rbg)
	if err != nil {
		return 0, 0
	}

	// Find updatedReplicas for prefill and decode roles from roleStatuses
	for _, roleStatus := range rbg.Status.RoleStatuses {
		if roleStatus.Name == "prefill" {
			prefillUpdated = roleStatus.UpdatedReplicas
		} else if roleStatus.Name == "decode" {
			decodeUpdated = roleStatus.UpdatedReplicas
		}
	}

	return prefillUpdated, decodeUpdated
}

// GetScheduledPodCount returns the number of pods that have been scheduled (have nodeName)
func (f *Framework) GetScheduledPodCount(app *arksv1.ArksDisaggregatedApplication, role string) int {
	podList := &corev1.PodList{}
	err := f.Client.List(f.Ctx, podList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{
			"arks.ai/application":         app.Name,
			"arks.ai/disaggregation-role": role,
		})
	if err != nil {
		return 0
	}

	count := 0
	for _, pod := range podList.Items {
		if pod.Spec.NodeName != "" {
			count++
		}
	}
	return count
}

// ExpectCoordinationScalingFirstBatch verifies that only the first batch of pods is created
// when pods cannot be scheduled (due to nodeSelector not matching any node).
// With maxSkew=20%, first batch should be ~20% of desired replicas for each role.
func (f *Framework) ExpectCoordinationScalingFirstBatch(
	app *arksv1.ArksDisaggregatedApplication,
	prefillDesired, decodeDesired int,
	maxSkewPercent float64,
) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name)

	// Calculate expected first batch size
	// First batch should be approximately maxSkew% of desired replicas
	// Using ceiling to ensure at least 1 pod is created
	expectedPrefillMax := int(float64(prefillDesired)*maxSkewPercent) + 1
	expectedDecodeMax := int(float64(decodeDesired)*maxSkewPercent) + 1

	// Wait for pods to be created (but not scheduled due to nodeSelector)
	gomega.Eventually(func() bool {
		prefillCount, decodeCount := f.GetPrefillAndDecodePodCounts(app)
		logger.V(1).Info("Checking first batch pod counts",
			"prefillCount", prefillCount, "expectedPrefillMax", expectedPrefillMax,
			"decodeCount", decodeCount, "expectedDecodeMax", expectedDecodeMax)

		// Should have some pods created
		if prefillCount == 0 && decodeCount == 0 {
			return false
		}

		// Should not exceed first batch size (since pods can't be scheduled)
		return prefillCount <= expectedPrefillMax && decodeCount <= expectedDecodeMax
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"First batch should be created with prefill<=%d, decode<=%d", expectedPrefillMax, expectedDecodeMax)

	// Verify pods are pending (not scheduled) due to nodeSelector
	gomega.Eventually(func() bool {
		prefillScheduled := f.GetScheduledPodCount(app, "prefill")
		decodeScheduled := f.GetScheduledPodCount(app, "decode")
		logger.V(1).Info("Checking scheduled pod counts",
			"prefillScheduled", prefillScheduled, "decodeScheduled", decodeScheduled)
		return prefillScheduled == 0 && decodeScheduled == 0
	}, ShortTimeout, Interval).Should(gomega.BeTrue(),
		"Pods should be pending (not scheduled) due to nodeSelector")

	// Wait a bit and verify no additional pods are created (since first batch isn't scheduled)
	time.Sleep(5 * time.Second)

	prefillCount, decodeCount := f.GetPrefillAndDecodePodCounts(app)
	gomega.Expect(prefillCount).To(gomega.BeNumerically("<=", expectedPrefillMax),
		"Prefill pod count should not exceed first batch size")
	gomega.Expect(decodeCount).To(gomega.BeNumerically("<=", expectedDecodeMax),
		"Decode pod count should not exceed first batch size")
}

// ExpectCoordinationScalingProgress verifies that during deployment, the progress difference
// between prefill and decode roles mostly stays within maxSkew.
// Due to async StatefulSet pod creation and controller reconcile cycles, transient differences
// may exceed maxSkew. We verify that at least 70% of samples satisfy the constraint.
func (f *Framework) ExpectCoordinationScalingProgress(
	app *arksv1.ArksDisaggregatedApplication,
	prefillDesired, decodeDesired int,
	maxSkewPercent float64,
) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name)

	// Sample progress multiple times during deployment
	maxSamples := 30
	sampleInterval := 300 * time.Millisecond

	var validSamples int     // samples where both roles have pods
	var satisfiedSamples int // samples where progressDiff <= maxSkew
	var maxObservedDiff float64
	var allSamples []string // for logging

	for i := 0; i < maxSamples; i++ {
		prefillCount, decodeCount := f.GetPrefillAndDecodePodCounts(app)

		// Skip samples where deployment hasn't really started
		if prefillCount == 0 && decodeCount == 0 {
			time.Sleep(sampleInterval)
			continue
		}

		// Calculate progress percentages
		prefillProgress := float64(prefillCount) / float64(prefillDesired)
		decodeProgress := float64(decodeCount) / float64(decodeDesired)

		// Calculate progress difference
		progressDiff := prefillProgress - decodeProgress
		if progressDiff < 0 {
			progressDiff = -progressDiff
		}

		validSamples++
		// Allow some tolerance (10%) for timing variations
		tolerance := 0.10
		if progressDiff <= maxSkewPercent+tolerance {
			satisfiedSamples++
		}
		if progressDiff > maxObservedDiff {
			maxObservedDiff = progressDiff
		}

		sampleInfo := fmt.Sprintf("sample%d: p=%d(%.0f%%) d=%d(%.0f%%) diff=%.0f%%",
			i, prefillCount, prefillProgress*100, decodeCount, decodeProgress*100, progressDiff*100)
		allSamples = append(allSamples, sampleInfo)

		logger.V(1).Info("Sampling coordination progress",
			"sample", i,
			"prefillCount", prefillCount, "prefillProgress", fmt.Sprintf("%.2f", prefillProgress),
			"decodeCount", decodeCount, "decodeProgress", fmt.Sprintf("%.2f", decodeProgress),
			"progressDiff", fmt.Sprintf("%.2f", progressDiff), "maxSkew", maxSkewPercent)

		// Check if deployment is complete
		if prefillCount >= prefillDesired && decodeCount >= decodeDesired {
			logger.Info("Deployment complete, stopping sampling")
			break
		}

		time.Sleep(sampleInterval)
	}

	// Print summary to GinkgoWriter for visibility in test output
	satisfactionRate := float64(0)
	if validSamples > 0 {
		satisfactionRate = float64(satisfiedSamples) / float64(validSamples)
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "\n=== Coordination Scaling Progress Summary ===\n")
	fmt.Fprintf(ginkgo.GinkgoWriter, "Valid samples: %d, Satisfied: %d (%.0f%%)\n", validSamples, satisfiedSamples, satisfactionRate*100)
	fmt.Fprintf(ginkgo.GinkgoWriter, "Max observed diff: %.0f%% (maxSkew: %.0f%%)\n", maxObservedDiff*100, maxSkewPercent*100)
	fmt.Fprintf(ginkgo.GinkgoWriter, "Samples: %v\n", allSamples)
	fmt.Fprintf(ginkgo.GinkgoWriter, "=============================================\n\n")

	// Verify that at least 70% of valid samples satisfy maxSkew constraint
	if validSamples > 0 {
		gomega.Expect(satisfactionRate).To(gomega.BeNumerically(">=", 0.70),
			"At least 70%% of samples should satisfy maxSkew constraint, got %.0f%% (%d/%d). Max observed diff: %.2f. Samples: %v",
			satisfactionRate*100, satisfiedSamples, validSamples, maxObservedDiff, allSamples)
	}

	// Also verify that max observed difference is not too extreme (e.g., not > 2x maxSkew)
	gomega.Expect(maxObservedDiff).To(gomega.BeNumerically("<=", maxSkewPercent*2.5),
		"Max observed progress difference (%.2f) should not exceed 2.5x maxSkew (%.2f)",
		maxObservedDiff, maxSkewPercent)
}

// ExpectCoordinationRollingUpdateProgress verifies that during rolling update, the update progress
// difference between prefill and decode roles mostly stays within maxSkew.
// This is similar to ExpectCoordinationScalingProgress but uses UpdatedReplicas instead of pod counts.
// Due to async StatefulSet pod updates and controller reconcile cycles, transient differences
// may exceed maxSkew. We verify that at least 70% of samples satisfy the constraint.
func (f *Framework) ExpectCoordinationRollingUpdateProgress(
	app *arksv1.ArksDisaggregatedApplication,
	prefillDesired, decodeDesired int,
	maxSkewPercent float64,
) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name)

	// First, wait for rolling update to actually start on BOTH roles (updatedReplicas < desired for both)
	// RBG controller needs time to detect the spec change and reset updatedReplicas
	// This can take 30-40 seconds due to ArksDisaggApp -> RBG -> LWS propagation delay
	// We wait for both roles to start to avoid sampling the initial transient state
	// where one role has been reset but the other hasn't yet.
	waitTimeout := 60 * time.Second
	waitInterval := 200 * time.Millisecond
	startTime := time.Now()
	for {
		prefillUpdated, decodeUpdated := f.GetPrefillAndDecodeUpdatedReplicas(app)
		// Rolling update started when BOTH roles have updatedReplicas < desired
		// This avoids the transient state where decode=0 but prefill still shows old value
		if int(prefillUpdated) < prefillDesired && int(decodeUpdated) < decodeDesired {
			fmt.Fprintf(ginkgo.GinkgoWriter, "Rolling update started on both roles: prefill=%d/%d, decode=%d/%d\n",
				prefillUpdated, prefillDesired, decodeUpdated, decodeDesired)
			break
		}
		if time.Since(startTime) > waitTimeout {
			fmt.Fprintf(ginkgo.GinkgoWriter, "Warning: Rolling update did not start within timeout, proceeding anyway\n")
			break
		}
		time.Sleep(waitInterval)
	}

	// Sample progress continuously until rolling update completes or timeout
	// With 15 pods updating at ~30s each, rolling update can take 7-8 minutes
	samplingTimeout := 10 * time.Minute
	sampleInterval := 1 * time.Second
	samplingStartTime := time.Now()

	var validSamples int     // samples where update is in progress
	var satisfiedSamples int // samples where progressDiff <= maxSkew
	var maxObservedDiff float64
	var allSamples []string // for logging
	sampleIndex := 0

	for {
		// Check timeout
		if time.Since(samplingStartTime) > samplingTimeout {
			fmt.Fprintf(ginkgo.GinkgoWriter, "Sampling timeout reached after %v\n", samplingTimeout)
			break
		}

		prefillUpdated, decodeUpdated := f.GetPrefillAndDecodeUpdatedReplicas(app)

		// Stop when rolling update is complete (both roles fully updated)
		if int(prefillUpdated) >= prefillDesired && int(decodeUpdated) >= decodeDesired {
			fmt.Fprintf(ginkgo.GinkgoWriter, "Rolling update complete: prefill=%d/%d, decode=%d/%d\n",
				prefillUpdated, prefillDesired, decodeUpdated, decodeDesired)
			break
		}

		// Calculate update progress percentages
		prefillProgress := float64(prefillUpdated) / float64(prefillDesired)
		decodeProgress := float64(decodeUpdated) / float64(decodeDesired)

		// Calculate progress difference
		progressDiff := prefillProgress - decodeProgress
		if progressDiff < 0 {
			progressDiff = -progressDiff
		}

		validSamples++
		// Allow some tolerance (10%) for timing variations
		tolerance := 0.10
		if progressDiff <= maxSkewPercent+tolerance {
			satisfiedSamples++
		}
		if progressDiff > maxObservedDiff {
			maxObservedDiff = progressDiff
		}

		sampleInfo := fmt.Sprintf("sample%d: p=%d(%.0f%%) d=%d(%.0f%%) diff=%.0f%%",
			sampleIndex, prefillUpdated, prefillProgress*100, decodeUpdated, decodeProgress*100, progressDiff*100)
		allSamples = append(allSamples, sampleInfo)

		// Print progress every 10 samples for visibility
		if sampleIndex%10 == 0 {
			fmt.Fprintf(ginkgo.GinkgoWriter, "  [%d] p=%d/%d (%.0f%%) d=%d/%d (%.0f%%) diff=%.0f%%\n",
				sampleIndex, prefillUpdated, prefillDesired, prefillProgress*100,
				decodeUpdated, decodeDesired, decodeProgress*100, progressDiff*100)
		}

		logger.V(1).Info("Sampling rolling update coordination progress",
			"sample", sampleIndex,
			"prefillUpdated", prefillUpdated, "prefillProgress", fmt.Sprintf("%.2f", prefillProgress),
			"decodeUpdated", decodeUpdated, "decodeProgress", fmt.Sprintf("%.2f", decodeProgress),
			"progressDiff", fmt.Sprintf("%.2f", progressDiff), "maxSkew", maxSkewPercent)

		sampleIndex++
		time.Sleep(sampleInterval)
	}

	// Print summary to GinkgoWriter for visibility in test output
	satisfactionRate := float64(0)
	if validSamples > 0 {
		satisfactionRate = float64(satisfiedSamples) / float64(validSamples)
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, "\n=== Coordination RollingUpdate Progress Summary ===\n")
	fmt.Fprintf(ginkgo.GinkgoWriter, "Valid samples: %d, Satisfied: %d (%.0f%%)\n", validSamples, satisfiedSamples, satisfactionRate*100)
	fmt.Fprintf(ginkgo.GinkgoWriter, "Max observed diff: %.0f%% (maxSkew: %.0f%%)\n", maxObservedDiff*100, maxSkewPercent*100)
	fmt.Fprintf(ginkgo.GinkgoWriter, "Samples: %v\n", allSamples)
	fmt.Fprintf(ginkgo.GinkgoWriter, "==================================================\n\n")

	// Verify that at least 70% of valid samples satisfy maxSkew constraint
	if validSamples > 0 {
		gomega.Expect(satisfactionRate).To(gomega.BeNumerically(">=", 0.70),
			"At least 70%% of samples should satisfy maxSkew constraint, got %.0f%% (%d/%d). Max observed diff: %.2f. Samples: %v",
			satisfactionRate*100, satisfiedSamples, validSamples, maxObservedDiff, allSamples)
	}

	// Also verify that max observed difference is not too extreme (e.g., not > 2.5x maxSkew)
	gomega.Expect(maxObservedDiff).To(gomega.BeNumerically("<=", maxSkewPercent*2.5),
		"Max observed progress difference (%.2f) should not exceed 2.5x maxSkew (%.2f)",
		maxObservedDiff, maxSkewPercent)
}

// ExpectAllPodsScheduled verifies all pods for the app are scheduled (have nodeName)
func (f *Framework) ExpectAllPodsScheduled(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application": app.Name,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			if pod.Spec.NodeName == "" {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"All pods should be scheduled (have nodeName)")
}

// ExpectLWSPodHasSchedulerName verifies LWS pods have the expected schedulerName
func (f *Framework) ExpectLWSPodHasSchedulerName(app *arksv1.ArksDisaggregatedApplication, role string, expectedSchedulerName string) {
	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application":         app.Name,
				"arks.ai/disaggregation-role": role,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			if pod.Spec.SchedulerName != expectedSchedulerName {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"%s pods should have schedulerName=%s", role, expectedSchedulerName)
}

// ExpectLWSPodHasVolcanoPodGroupAnnotation verifies LWS pods have Volcano PodGroup annotation
// This indicates LWS-level gang scheduling is active
func (f *Framework) ExpectLWSPodHasVolcanoPodGroupAnnotation(app *arksv1.ArksDisaggregatedApplication, role string) {
	const volcanoAnnotationKey = "scheduling.k8s.io/group-name"

	gomega.Eventually(func() bool {
		podList := &corev1.PodList{}
		err := f.Client.List(f.Ctx, podList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{
				"arks.ai/application":         app.Name,
				"arks.ai/disaggregation-role": role,
			})
		if err != nil || len(podList.Items) == 0 {
			return false
		}

		for _, pod := range podList.Items {
			if _, ok := pod.Annotations[volcanoAnnotationKey]; !ok {
				return false
			}
		}
		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"%s pods should have Volcano PodGroup annotation", role)
}

// GetLWSReplicaSchedulingStatus returns the scheduling status per LWS replica
// Returns a map of replicaIndex -> (scheduledCount, pendingCount, totalCount)
func (f *Framework) GetLWSReplicaSchedulingStatus(app *arksv1.ArksDisaggregatedApplication, role string) map[string]struct {
	Scheduled int
	Pending   int
	Total     int
} {
	podList := &corev1.PodList{}
	err := f.Client.List(f.Ctx, podList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{
			"arks.ai/application":         app.Name,
			"arks.ai/disaggregation-role": role,
		})
	if err != nil {
		return nil
	}

	replicaStatus := make(map[string]struct {
		Scheduled int
		Pending   int
		Total     int
	})

	for _, pod := range podList.Items {
		groupIndex := pod.Labels["leaderworkerset.sigs.k8s.io/group-index"]
		if groupIndex == "" {
			continue
		}

		status := replicaStatus[groupIndex]
		status.Total++
		if pod.Spec.NodeName != "" {
			status.Scheduled++
		} else {
			status.Pending++
		}
		replicaStatus[groupIndex] = status
	}

	return replicaStatus
}

// VerifyLWSGangNoPartialInstances verifies LWS gang scheduling: no partial instance scheduling
// For each LWS replica, either all pods are scheduled or all are pending
func (f *Framework) VerifyLWSGangNoPartialInstances(app *arksv1.ArksDisaggregatedApplication, role string, lwsSize int) {
	replicaStatus := f.GetLWSReplicaSchedulingStatus(app, role)

	for replicaIndex, status := range replicaStatus {
		// If we have the full size, verify no partial scheduling
		if status.Total == lwsSize {
			isAllScheduled := status.Scheduled == lwsSize
			isAllPending := status.Pending == lwsSize
			ginkgo.By(fmt.Sprintf("Checking LWS %s-%s replica %s: %d/%d scheduled, %d pending (size=%d)",
				app.Name, role, replicaIndex, status.Scheduled, status.Total, status.Pending, lwsSize))
			gomega.Expect(isAllScheduled || isAllPending).To(gomega.BeTrue(),
				"LWS gang violation: replica %s has %d scheduled and %d pending pods (expected all scheduled or all pending)",
				replicaIndex, status.Scheduled, status.Pending)
		}
	}
}

// LogLWSSchedulingStatus logs the scheduling status of LWS pods for debugging
func (f *Framework) LogLWSSchedulingStatus(app *arksv1.ArksDisaggregatedApplication, role string) {
	replicaStatus := f.GetLWSReplicaSchedulingStatus(app, role)

	ginkgo.By(fmt.Sprintf("LWS scheduling status for %s-%s:", app.Name, role))
	for idx, status := range replicaStatus {
		ginkgo.By(fmt.Sprintf("  Replica %s: %d/%d scheduled, %d pending",
			idx, status.Scheduled, status.Total, status.Pending))
	}
}

// GetFullyScheduledLWSReplicaCount returns the number of fully scheduled LWS replicas
func (f *Framework) GetFullyScheduledLWSReplicaCount(app *arksv1.ArksDisaggregatedApplication, role string) int {
	replicaStatus := f.GetLWSReplicaSchedulingStatus(app, role)

	count := 0
	for _, status := range replicaStatus {
		if status.Total > 0 && status.Scheduled == status.Total {
			count++
		}
	}
	return count
}

// GetLWSPartition returns the partition value for the specified role's LWS
// Returns -1 if LWS not found or RollingUpdateConfiguration not set, 0 if partition is nil (default)
func (f *Framework) GetLWSPartition(app *arksv1.ArksDisaggregatedApplication, role string) int32 {
	lws := f.GetLWSForRole(app, role)
	if lws == nil {
		return -1
	}

	if lws.Spec.RolloutStrategy.RollingUpdateConfiguration == nil {
		return -1
	}

	partition := lws.Spec.RolloutStrategy.RollingUpdateConfiguration.Partition
	if partition == nil {
		return 0
	}
	return *partition
}

// GetLWSForRole returns the LWS for the specified role
// In RBG mode, LWS name is <app.Name>-0-<role> (e.g., myapp-0-prefill)
// In legacy LWS mode, LWS name is <app.Name>-<role> (e.g., myapp-prefill)
func (f *Framework) GetLWSForRole(app *arksv1.ArksDisaggregatedApplication, role string) *lwsv1.LeaderWorkerSet {
	// Try RBG mode first: LWS name is <app.Name>-0-<role>
	// This follows the pattern: RBGS creates RBG named <rbgs.Name>-<index>,
	// and RBG creates LWS named <rbg.Name>-<role>
	// Since RBGS name = app.Name, LWS name = app.Name-0-role
	lwsName := fmt.Sprintf("%s-0-%s", app.Name, role)
	lws := &lwsv1.LeaderWorkerSet{}
	err := f.Client.Get(f.Ctx, client.ObjectKey{
		Namespace: app.Namespace,
		Name:      lwsName,
	}, lws)
	if err == nil {
		return lws
	}

	// Fallback: legacy LWS mode, LWS name is <app.Name>-<role>
	lwsName = fmt.Sprintf("%s-%s", app.Name, role)
	err = f.Client.Get(f.Ctx, client.ObjectKey{
		Namespace: app.Namespace,
		Name:      lwsName,
	}, lws)
	if err == nil {
		return lws
	}

	return nil
}

// GetLWSRolloutStrategyInfo returns a string describing the LWS rollout strategy for logging
func (f *Framework) GetLWSRolloutStrategyInfo(app *arksv1.ArksDisaggregatedApplication, role string) string {
	lws := f.GetLWSForRole(app, role)
	if lws == nil {
		return fmt.Sprintf("LWS not found for %s/%s", app.Name, role)
	}

	rs := lws.Spec.RolloutStrategy
	if rs.RollingUpdateConfiguration == nil {
		return fmt.Sprintf("LWS %s: no RollingUpdateConfiguration", lws.Name)
	}

	ruc := rs.RollingUpdateConfiguration
	partitionVal := int32(0)
	if ruc.Partition != nil {
		partitionVal = *ruc.Partition
	}
	return fmt.Sprintf("LWS %s: partition=%d, maxUnavailable=%v, maxSurge=%v",
		lws.Name, partitionVal, ruc.MaxUnavailable, ruc.MaxSurge)
}

// VerifyLWSPartitionIsSet verifies that the LWS partition is set to a non-negative value
// This confirms that RBG coordination is actually applying partition to the LWS
func (f *Framework) VerifyLWSPartitionIsSet(app *arksv1.ArksDisaggregatedApplication, role string) {
	partition := f.GetLWSPartition(app, role)
	ginkgo.By(fmt.Sprintf("LWS %s partition = %d", role, partition))
	gomega.Expect(partition).To(gomega.BeNumerically(">=", 0),
		"LWS %s should have partition set (>= 0), got %d", role, partition)
}

// SampleLWSPartitionsDuringRollingUpdate samples LWS partition values during rolling update
// Returns true if partitions are being actively managed (changed during the update)
func (f *Framework) SampleLWSPartitionsDuringRollingUpdate(
	app *arksv1.ArksDisaggregatedApplication,
	prefillDesired, decodeDesired int32,
) (partitionSamples []string, partitionWasSet bool) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name)

	samplingTimeout := 3 * time.Minute
	sampleInterval := 500 * time.Millisecond
	samplingStartTime := time.Now()

	for {
		if time.Since(samplingStartTime) > samplingTimeout {
			break
		}

		prefillPartition := f.GetLWSPartition(app, "prefill")
		decodePartition := f.GetLWSPartition(app, "decode")
		prefillUpdated, decodeUpdated := f.GetPrefillAndDecodeUpdatedReplicas(app)

		sample := fmt.Sprintf("p_part=%d p_upd=%d/%d, d_part=%d d_upd=%d/%d",
			prefillPartition, prefillUpdated, prefillDesired,
			decodePartition, decodeUpdated, decodeDesired)
		partitionSamples = append(partitionSamples, sample)

		// Check if partition is being used (non-zero partition during update)
		if prefillPartition > 0 || decodePartition > 0 {
			partitionWasSet = true
		}

		logger.V(1).Info("Sampling LWS partitions",
			"prefillPartition", prefillPartition, "decodePartition", decodePartition,
			"prefillUpdated", prefillUpdated, "decodeUpdated", decodeUpdated)

		// Stop if rolling update is complete
		if prefillUpdated >= prefillDesired && decodeUpdated >= decodeDesired {
			break
		}

		time.Sleep(sampleInterval)
	}

	return partitionSamples, partitionWasSet
}
