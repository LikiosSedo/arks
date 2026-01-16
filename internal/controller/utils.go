package controller

import (
	rbgv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

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
