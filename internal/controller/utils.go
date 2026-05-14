package controller

import (
	"encoding/json"
	"reflect"

	rbgv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

// rbgsSpecSemanticallyEqual compares two RoleBasedGroupSetSpec by their JSON
// serialization. This avoids spurious PATCH requests caused by Go type-level
// differences (pointer vs value, nil vs empty slice) that serialize to
// identical JSON. Without this check, upgrading Arks across RBG API versions
// (e.g., v0.5.0 → v0.6.0 pointer type changes) can trigger a reconcile
// cascade that recreates downstream LWS pods.
func rbgsSpecSemanticallyEqual(a, b rbgv1alpha1.RoleBasedGroupSetSpec) bool {
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	var aMap, bMap interface{}
	if json.Unmarshal(aJSON, &aMap) != nil || json.Unmarshal(bJSON, &bMap) != nil {
		return false
	}
	return reflect.DeepEqual(aMap, bMap)
}

// isArksAppReady checks whether an ArksApplication has all components ready.
// Used by both the application controller (phase update) and the endpoint controller.
func isArksAppReady(app *arksv1.ArksApplication) bool {
	if app.Status.TrafficTarget == arksv1.ArksApplicationTrafficTargetPending {
		return false
	}
	switch app.Status.Mode {
	case "", arksv1.ArksApplicationModeUnified:
		routerNeeded := app.Spec.Router != nil ||
			app.Status.TrafficTarget == arksv1.ArksApplicationTrafficTargetRouter ||
			getApplicationMode(app) == arksv1.ArksApplicationModeDisaggregated
		if routerNeeded {
			return app.Status.Unified.Replicas > 0 &&
				app.Status.Unified.ReadyReplicas == app.Status.Unified.Replicas &&
				app.Status.Router.ReadyReplicas > 0 &&
				checkApplicationCondition(app, arksv1.ArksApplicationTrafficTargetReady)
		}
		return app.Status.Unified.Replicas > 0 &&
			app.Status.Unified.ReadyReplicas == app.Status.Unified.Replicas
	case arksv1.ArksApplicationModeDisaggregated:
		return app.Status.Router.ReadyReplicas > 0 &&
			app.Status.Prefill.Replicas > 0 &&
			app.Status.Prefill.ReadyReplicas == app.Status.Prefill.Replicas &&
			app.Status.Decode.Replicas > 0 &&
			app.Status.Decode.ReadyReplicas == app.Status.Decode.Replicas &&
			checkApplicationCondition(app, arksv1.ArksApplicationTrafficTargetReady)
	default:
		return false
	}
}

// hasCustomRouter returns true when the user provides both a custom router
// image (top-level spec.routerImage) and a command override, allowing non-sglang
// runtimes to use a router.
func hasCustomRouter(application *arksv1.ArksApplication) bool {
	if application.Spec.Router == nil {
		return false
	}
	return application.Spec.RouterImage != "" && len(application.Spec.Router.CommandOverride) > 0
}

// convertToRbgPodGroupPolicy converts the legacy ArksDisaggregatedApplication
// PodGroupPolicy into the RBG-side PodGroupPolicy. Kept for the legacy CRD only.
func convertToRbgPodGroupPolicy(podGroupPolicy *arksv1.PodGroupPolicy) *rbgv1alpha1.PodGroupPolicy {
	var rbgPodGroupPolicy *rbgv1alpha1.PodGroupPolicy
	if podGroupPolicy != nil {
		rbgPodGroupPolicy = &rbgv1alpha1.PodGroupPolicy{}
		if podGroupPolicy.KubeScheduling != nil {
			rbgPodGroupPolicy.KubeScheduling = &rbgv1alpha1.KubeSchedulingPodGroupPolicySource{
				ScheduleTimeoutSeconds: podGroupPolicy.KubeScheduling.ScheduleTimeoutSeconds,
			}
		}
		if podGroupPolicy.VolcanoScheduling != nil {
			rbgPodGroupPolicy.VolcanoScheduling = &rbgv1alpha1.VolcanoSchedulingPodGroupPolicySource{
				PriorityClassName: podGroupPolicy.VolcanoScheduling.PriorityClassName,
				Queue:             podGroupPolicy.VolcanoScheduling.Queue,
			}
		}
	}
	return rbgPodGroupPolicy
}

// convertArksAppPodGroupPolicy converts the new ArksApplication-owned
// PodGroupPolicy into the RBG-side PodGroupPolicy.
func convertArksAppPodGroupPolicy(podGroupPolicy *arksv1.ArksPodGroupPolicy) *rbgv1alpha1.PodGroupPolicy {
	var rbgPodGroupPolicy *rbgv1alpha1.PodGroupPolicy
	if podGroupPolicy != nil {
		rbgPodGroupPolicy = &rbgv1alpha1.PodGroupPolicy{}
		if podGroupPolicy.KubeScheduling != nil {
			rbgPodGroupPolicy.KubeScheduling = &rbgv1alpha1.KubeSchedulingPodGroupPolicySource{
				ScheduleTimeoutSeconds: podGroupPolicy.KubeScheduling.ScheduleTimeoutSeconds,
			}
		}
		if podGroupPolicy.VolcanoScheduling != nil {
			rbgPodGroupPolicy.VolcanoScheduling = &rbgv1alpha1.VolcanoSchedulingPodGroupPolicySource{
				PriorityClassName: podGroupPolicy.VolcanoScheduling.PriorityClassName,
				Queue:             podGroupPolicy.VolcanoScheduling.Queue,
			}
		}
	}
	return rbgPodGroupPolicy
}

// buildCoordinationRequirements constructs coordination configuration for RBG.
// This function converts arks CoordinationPolicy to RBG's Coordination requirements,
// supporting both Scaling coordination (for initial deployment/scale-up) and
// RollingUpdate coordination (for rolling updates).
func buildCoordinationRequirements(coordinationPolicy *arksv1.CoordinationPolicy) []rbgv1alpha1.Coordination {
	if coordinationPolicy == nil {
		return nil
	}

	// No coordination if neither scaling nor rollingUpdate is configured
	if coordinationPolicy.Scaling == nil && coordinationPolicy.RollingUpdate == nil {
		return nil
	}

	coord := rbgv1alpha1.Coordination{
		Name:     "pd-coordination",
		Roles:    []string{"prefill", "decode"}, // Only coordinate prefill and decode, not router
		Strategy: &rbgv1alpha1.CoordinationStrategy{},
	}

	// Build Scaling coordination strategy
	if coordinationPolicy.Scaling != nil {
		maxSkew := coordinationPolicy.Scaling.MaxSkew
		if maxSkew == "" {
			maxSkew = "10%"
		}

		progression := rbgv1alpha1.ProgressionType(coordinationPolicy.Scaling.Progression)
		if progression == "" {
			progression = rbgv1alpha1.OrderScheduled
		}

		coord.Strategy.Scaling = &rbgv1alpha1.CoordinationScaling{
			MaxSkew:     &maxSkew,
			Progression: &progression,
		}
	}

	// Build RollingUpdate coordination strategy
	if coordinationPolicy.RollingUpdate != nil {
		coord.Strategy.RollingUpdate = &rbgv1alpha1.CoordinationRollingUpdate{}

		if coordinationPolicy.RollingUpdate.MaxSkew != "" {
			maxSkew := coordinationPolicy.RollingUpdate.MaxSkew
			coord.Strategy.RollingUpdate.MaxSkew = &maxSkew
		}
		if coordinationPolicy.RollingUpdate.MaxUnavailable != "" {
			maxUnavailable := coordinationPolicy.RollingUpdate.MaxUnavailable
			coord.Strategy.RollingUpdate.MaxUnavailable = &maxUnavailable
		}
		if coordinationPolicy.RollingUpdate.Partition != "" {
			partition := coordinationPolicy.RollingUpdate.Partition
			coord.Strategy.RollingUpdate.Partition = &partition
		}
	}

	return []rbgv1alpha1.Coordination{coord}
}
