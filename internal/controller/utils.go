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
