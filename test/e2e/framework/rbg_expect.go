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
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	rbgv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

const (
	// ResourceNvidiaGPU is the GPU resource name
	ResourceNvidiaGPU corev1.ResourceName = "nvidia.com/gpu"
)

// ExpectRBGSExists verifies the underlying RoleBasedGroupSet exists for the app
func (f *Framework) ExpectRBGSExists(app *arksv1.ArksDisaggregatedApplication) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	gomega.Eventually(func() bool {
		rbgsList := &rbgv1alpha1.RoleBasedGroupSetList{}
		err := f.Client.List(f.Ctx, rbgsList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{arksv1.ArksControllerKeyApplication: app.Name})
		if err != nil {
			logger.Error(err, "Failed to list RoleBasedGroupSets")
			return false
		}
		return len(rbgsList.Items) > 0
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RoleBasedGroupSet should exist for app %s/%s", app.Namespace, app.Name)
}

// ExpectRBGSDeleted verifies the underlying RoleBasedGroupSet is deleted
func (f *Framework) ExpectRBGSDeleted(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		rbgsList := &rbgv1alpha1.RoleBasedGroupSetList{}
		err := f.Client.List(f.Ctx, rbgsList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{arksv1.ArksControllerKeyApplication: app.Name})
		if err != nil {
			return false
		}
		return len(rbgsList.Items) == 0
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RoleBasedGroupSet should be deleted for app %s/%s", app.Namespace, app.Name)
}

// ExpectRBGSReady verifies RBGS is ready with expected roles
func (f *Framework) ExpectRBGSReady(app *arksv1.ArksDisaggregatedApplication) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	gomega.Eventually(func() bool {
		rbgsList := &rbgv1alpha1.RoleBasedGroupSetList{}
		err := f.Client.List(f.Ctx, rbgsList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{arksv1.ArksControllerKeyApplication: app.Name})
		if err != nil {
			logger.Error(err, "Failed to list RoleBasedGroupSets")
			return false
		}

		if len(rbgsList.Items) == 0 {
			return false
		}

		rbgs := &rbgsList.Items[0]
		// ArksDisaggregatedApplication creates 3 roles: scheduler, prefill, decode
		if len(rbgs.Spec.Template.Roles) != 3 {
			logger.V(1).Info("RBGS does not have expected roles",
				"roleCount", len(rbgs.Spec.Template.Roles))
			return false
		}

		return true
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBGS should have 3 roles for app %s/%s", app.Namespace, app.Name)
}

// ExpectRBGSRoleCount verifies RBGS has the expected number of roles
func (f *Framework) ExpectRBGSRoleCount(app *arksv1.ArksDisaggregatedApplication, expectedRoles int) {
	gomega.Eventually(func() int {
		rbgsList := &rbgv1alpha1.RoleBasedGroupSetList{}
		err := f.Client.List(f.Ctx, rbgsList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{arksv1.ArksControllerKeyApplication: app.Name})
		if err != nil {
			return -1
		}

		if len(rbgsList.Items) == 0 {
			return 0
		}

		return len(rbgsList.Items[0].Spec.Template.Roles)
	}, Timeout, Interval).Should(gomega.Equal(expectedRoles),
		"RBGS should have %d roles", expectedRoles)
}

// GetRBGS fetches the RoleBasedGroupSet for the given app
// Returns nil if RBGS doesn't exist yet (for use in Eventually blocks)
func (f *Framework) GetRBGS(app *arksv1.ArksDisaggregatedApplication) *rbgv1alpha1.RoleBasedGroupSet {
	rbgsList := &rbgv1alpha1.RoleBasedGroupSetList{}
	err := f.Client.List(f.Ctx, rbgsList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{arksv1.ArksControllerKeyApplication: app.Name})
	if err != nil || len(rbgsList.Items) == 0 {
		return nil
	}
	return &rbgsList.Items[0]
}

// ExpectRBGExists verifies an underlying RoleBasedGroup exists
// Note: RBG is created by RBGS controller, so we query by RBGS name label
func (f *Framework) ExpectRBGExists(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		rbgList := &rbgv1alpha1.RoleBasedGroupList{}
		// RBG uses "rolebasedgroupset.workloads.x-k8s.io/name" label, not arks label
		err := f.Client.List(f.Ctx, rbgList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{"rolebasedgroupset.workloads.x-k8s.io/name": app.Name})
		if err != nil {
			return false
		}
		return len(rbgList.Items) > 0
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RoleBasedGroup should exist for app %s/%s", app.Namespace, app.Name)
}

// ExpectRBGDeleted verifies the underlying RoleBasedGroup is deleted
func (f *Framework) ExpectRBGDeleted(app *arksv1.ArksDisaggregatedApplication) {
	gomega.Eventually(func() bool {
		rbgList := &rbgv1alpha1.RoleBasedGroupList{}
		// RBG uses "rolebasedgroupset.workloads.x-k8s.io/name" label
		err := f.Client.List(f.Ctx, rbgList,
			client.InNamespace(app.Namespace),
			client.MatchingLabels{"rolebasedgroupset.workloads.x-k8s.io/name": app.Name})
		if err != nil {
			return false
		}
		return len(rbgList.Items) == 0
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RoleBasedGroup should be deleted for app %s/%s", app.Namespace, app.Name)
}

// GetRBGs fetches all RoleBasedGroups for the given app
// Note: RBG is created by RBGS controller with its own labels
func (f *Framework) GetRBGs(app *arksv1.ArksDisaggregatedApplication) []rbgv1alpha1.RoleBasedGroup {
	rbgList := &rbgv1alpha1.RoleBasedGroupList{}
	// RBG uses "rolebasedgroupset.workloads.x-k8s.io/name" label
	err := f.Client.List(f.Ctx, rbgList,
		client.InNamespace(app.Namespace),
		client.MatchingLabels{"rolebasedgroupset.workloads.x-k8s.io/name": app.Name})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return rbgList.Items
}

// ExpectRBGRoleExists verifies a specific role exists in the RBG
func (f *Framework) ExpectRBGRoleExists(app *arksv1.ArksDisaggregatedApplication, roleName string) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		for _, role := range rbgs.Spec.Template.Roles {
			if role.Name == roleName {
				return true
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"Role %s should exist in RBGS for app %s/%s", roleName, app.Namespace, app.Name)
}

// ExpectRBGRoleReplicas verifies a specific role has expected replicas
func (f *Framework) ExpectRBGRoleReplicas(app *arksv1.ArksDisaggregatedApplication, roleName string, expected int32) {
	gomega.Eventually(func() int32 {
		rbgs := f.GetRBGS(app)
		for _, role := range rbgs.Spec.Template.Roles {
			if role.Name == roleName && role.Replicas != nil {
				return *role.Replicas
			}
		}
		return -1
	}, Timeout, Interval).Should(gomega.Equal(expected),
		"Role %s should have %d replicas", roleName, expected)
}

// ExpectArksModelExists verifies the ArksModel exists
func (f *Framework) ExpectArksModelExists(modelName, namespace string) {
	gomega.Eventually(func() bool {
		model := &arksv1.ArksModel{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      modelName,
			Namespace: namespace,
		}, model)
		return err == nil
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"ArksModel %s/%s should exist", namespace, modelName)
}

// ExpectArksModelReady verifies the ArksModel is in Ready phase
func (f *Framework) ExpectArksModelReady(model *arksv1.ArksModel) {
	gomega.Eventually(func() bool {
		newModel := &arksv1.ArksModel{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      model.Name,
			Namespace: model.Namespace,
		}, newModel)
		if err != nil {
			return false
		}
		return newModel.Status.Phase == string(arksv1.ArksModelReady)
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"ArksModel %s/%s should be Ready", model.Namespace, model.Name)
}

// ExpectArksModelDeleted verifies the ArksModel is deleted
func (f *Framework) ExpectArksModelDeleted(model *arksv1.ArksModel) {
	gomega.Eventually(func() bool {
		newModel := &arksv1.ArksModel{}
		err := f.Client.Get(f.Ctx, client.ObjectKey{
			Name:      model.Name,
			Namespace: model.Namespace,
		}, newModel)
		return apierrors.IsNotFound(err)
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"ArksModel %s/%s should be deleted", model.Namespace, model.Name)
}

// ExpectRBGSRoleHasGPUResources verifies that a specific role in RBGS has the expected GPU count
func (f *Framework) ExpectRBGSRoleHasGPUResources(app *arksv1.ArksDisaggregatedApplication, roleName string, expectedGPUCount int64) {
	gomega.Eventually(func() int64 {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return -1
		}

		for _, role := range rbgs.Spec.Template.Roles {
			if role.Name == roleName {
				// Check the first container's resources in the pod template
				if role.Template == nil || len(role.Template.Spec.Containers) == 0 {
					return -1
				}
				container := role.Template.Spec.Containers[0]
				gpuLimit := container.Resources.Limits[ResourceNvidiaGPU]
				return gpuLimit.Value()
			}
		}
		return -1
	}, Timeout, Interval).Should(gomega.Equal(expectedGPUCount),
		"Role %s should have %d GPU resources", roleName, expectedGPUCount)
}

// ExpectRBGSRoleHasResourceQuantity verifies that a specific role has the expected resource quantity
func (f *Framework) ExpectRBGSRoleHasResourceQuantity(app *arksv1.ArksDisaggregatedApplication, roleName string, resourceName corev1.ResourceName, expected resource.Quantity) {
	gomega.Eventually(func() bool {
		rbgs := f.GetRBGS(app)
		if rbgs == nil {
			return false
		}

		for _, role := range rbgs.Spec.Template.Roles {
			if role.Name == roleName {
				if role.Template == nil || len(role.Template.Spec.Containers) == 0 {
					return false
				}
				container := role.Template.Spec.Containers[0]
				actual := container.Resources.Limits[resourceName]
				return actual.Equal(expected)
			}
		}
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"Role %s should have %s=%s", roleName, resourceName, expected.String())
}

// ExpectRBGCoordinationScaling verifies RBG (child resource, not RBGS) has CoordinationScaling with expected values
// This checks the actual RBG spec to verify if RBGS controller properly syncs CoordinationRequirements updates
func (f *Framework) ExpectRBGCoordinationScaling(app *arksv1.ArksDisaggregatedApplication, coordinationName string, expectedMaxSkew string) {
	gomega.Eventually(func() bool {
		rbgList := f.GetRBGs(app)
		if len(rbgList) == 0 {
			ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationScaling: No RBGs found for app %s\n", app.Name)
			return false
		}

		// Check first RBG (there should be only one per RBGS in most cases)
		rbg := rbgList[0]
		ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationScaling: Found RBG %s, coordinationCount=%d\n", rbg.Name, len(rbg.Spec.CoordinationRequirements))
		for _, coord := range rbg.Spec.CoordinationRequirements {
			ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationScaling: Checking coordination name=%s, target=%s\n", coord.Name, coordinationName)
			if coord.Name == coordinationName {
				if coord.Strategy == nil || coord.Strategy.Scaling == nil {
					ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationScaling: RBG coordination %s has no scaling strategy\n", coordinationName)
					return false
				}
				if coord.Strategy.Scaling.MaxSkew == nil {
					ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationScaling: RBG coordination %s scaling has no MaxSkew\n", coordinationName)
					return false
				}
				actualMaxSkew := *coord.Strategy.Scaling.MaxSkew
				ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationScaling: coordination=%s, actualMaxSkew=%s, expectedMaxSkew=%s\n", coordinationName, actualMaxSkew, expectedMaxSkew)
				return actualMaxSkew == expectedMaxSkew
			}
		}
		ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationScaling: RBG coordination %s not found, total coordinations=%d\n", coordinationName, len(rbg.Spec.CoordinationRequirements))
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBG coordination %s should have maxSkew=%s", coordinationName, expectedMaxSkew)
}

// ExpectRBGCoordinationRollingUpdate verifies RBG (child resource) has CoordinationRollingUpdate with expected values
func (f *Framework) ExpectRBGCoordinationRollingUpdate(app *arksv1.ArksDisaggregatedApplication, coordinationName string, expectedMaxSkew, expectedMaxUnavailable, expectedPartition string) {
	gomega.Eventually(func() bool {
		rbgList := f.GetRBGs(app)
		if len(rbgList) == 0 {
			ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: No RBGs found for app %s\n", app.Name)
			return false
		}

		rbg := rbgList[0]
		ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: Found RBG %s, coordinationCount=%d\n", rbg.Name, len(rbg.Spec.CoordinationRequirements))
		for _, coord := range rbg.Spec.CoordinationRequirements {
			if coord.Name == coordinationName {
				if coord.Strategy == nil || coord.Strategy.RollingUpdate == nil {
					ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: RBG coordination %s has no RollingUpdate strategy\n", coordinationName)
					return false
				}
				ru := coord.Strategy.RollingUpdate
				// Check MaxSkew if expected
				if expectedMaxSkew != "" {
					if ru.MaxSkew == nil || *ru.MaxSkew != expectedMaxSkew {
						ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: MaxSkew mismatch, expected=%s, actual=%v\n", expectedMaxSkew, ru.MaxSkew)
						return false
					}
				}
				// Check MaxUnavailable if expected
				if expectedMaxUnavailable != "" {
					if ru.MaxUnavailable == nil || *ru.MaxUnavailable != expectedMaxUnavailable {
						ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: MaxUnavailable mismatch, expected=%s, actual=%v\n", expectedMaxUnavailable, ru.MaxUnavailable)
						return false
					}
				}
				// Check Partition if expected
				if expectedPartition != "" {
					if ru.Partition == nil || *ru.Partition != expectedPartition {
						ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: Partition mismatch, expected=%s, actual=%v\n", expectedPartition, ru.Partition)
						return false
					}
				}
				ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: RBG coordination %s RollingUpdate verified\n", coordinationName)
				return true
			}
		}
		ginkgo.GinkgoWriter.Printf("[DEBUG] ExpectRBGCoordinationRollingUpdate: RBG coordination %s not found\n", coordinationName)
		return false
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBG coordination %s should have RollingUpdate with maxSkew=%s, maxUnavailable=%s, partition=%s",
		coordinationName, expectedMaxSkew, expectedMaxUnavailable, expectedPartition)
}

// ExpectRBGNoCoordinationRequirements verifies RBG (child resource) has no CoordinationRequirements
func (f *Framework) ExpectRBGNoCoordinationRequirements(app *arksv1.ArksDisaggregatedApplication) {
	logger := log.FromContext(f.Ctx).WithValues("app", app.Name, "namespace", app.Namespace)

	gomega.Eventually(func() bool {
		rbgList := f.GetRBGs(app)
		if len(rbgList) == 0 {
			return false
		}

		rbg := rbgList[0]
		hasCoord := len(rbg.Spec.CoordinationRequirements) > 0
		logger.V(1).Info("RBG CoordinationRequirements check", "hasCoordinationRequirements", hasCoord)
		return !hasCoord
	}, Timeout, Interval).Should(gomega.BeTrue(),
		"RBG should have no CoordinationRequirements")
}
