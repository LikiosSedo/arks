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

package workloads

import (
	"context"
	"fmt"
	"reflect"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

// LWSChecker implements WorkloadChecker for LeaderWorkerSet workloads
type LWSChecker struct {
	ctx       context.Context
	client    client.Client
	namespace string
}

// NewLWSChecker creates a new LWSChecker
func NewLWSChecker(ctx context.Context, c client.Client, namespace string) *LWSChecker {
	return &LWSChecker{
		ctx:       ctx,
		client:    c,
		namespace: namespace,
	}
}

// getLWS retrieves the LeaderWorkerSet for the given app and role
func (l *LWSChecker) getLWS(appName, roleName string) (*lwsv1.LeaderWorkerSet, error) {
	lwsList := &lwsv1.LeaderWorkerSetList{}
	err := l.client.List(l.ctx, lwsList,
		client.InNamespace(l.namespace),
		client.MatchingLabels{
			arksv1.ArksControllerKeyApplication:        appName,
			arksv1.ArksControllerKeyDisaggregationRole: roleName,
		})
	if err != nil {
		return nil, err
	}

	if len(lwsList.Items) == 0 {
		return nil, fmt.Errorf("no LeaderWorkerSet found for app %s role %s", appName, roleName)
	}

	return &lwsList.Items[0], nil
}

// ExpectWorkloadExists implements WorkloadChecker
func (l *LWSChecker) ExpectWorkloadExists(appName, roleName string) error {
	_, err := l.getLWS(appName, roleName)
	return err
}

// ExpectWorkloadReady implements WorkloadChecker
func (l *LWSChecker) ExpectWorkloadReady(appName, roleName string, replicas int32) error {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
		return err
	}

	if lws.Status.ReadyReplicas != replicas {
		return fmt.Errorf("LWS %s/%s ready replicas: expected %d, got %d",
			l.namespace, lws.Name, replicas, lws.Status.ReadyReplicas)
	}
	return nil
}

// ExpectWorkloadDeleted implements WorkloadChecker
func (l *LWSChecker) ExpectWorkloadDeleted(appName, roleName string) error {
	_, err := l.getLWS(appName, roleName)
	if err != nil {
		// Expected to not find the LWS
		return nil
	}
	return fmt.Errorf("LeaderWorkerSet for app %s role %s still exists", appName, roleName)
}

// ExpectContainerImage implements WorkloadChecker
func (l *LWSChecker) ExpectContainerImage(appName, roleName, containerName, expectedImage string) error {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
		return err
	}

	for _, container := range lws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.Containers {
		if container.Name == containerName {
			if container.Image != expectedImage {
				return fmt.Errorf("container %s image: expected %s, got %s",
					containerName, expectedImage, container.Image)
			}
			return nil
		}
	}

	return fmt.Errorf("container %s not found in LWS %s", containerName, lws.Name)
}

// ExpectContainerResources implements WorkloadChecker
func (l *LWSChecker) ExpectContainerResources(appName, roleName, containerName string, expected corev1.ResourceRequirements) error {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
		return err
	}

	for _, container := range lws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.Containers {
		if container.Name == containerName {
			if !reflect.DeepEqual(container.Resources, expected) {
				return fmt.Errorf("container %s resources mismatch: expected %v, got %v",
					containerName, expected, container.Resources)
			}
			return nil
		}
	}

	return fmt.Errorf("container %s not found in LWS %s", containerName, lws.Name)
}

// ExpectContainerEnv implements WorkloadChecker
func (l *LWSChecker) ExpectContainerEnv(appName, roleName, containerName string, expectedEnv []corev1.EnvVar) error {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
		return err
	}

	for _, container := range lws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.Containers {
		if container.Name == containerName {
			// Check that all expected env vars exist
			for _, expected := range expectedEnv {
				found := false
				for _, actual := range container.Env {
					if actual.Name == expected.Name && actual.Value == expected.Value {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("env var %s=%s not found in container %s",
						expected.Name, expected.Value, containerName)
				}
			}
			return nil
		}
	}

	return fmt.Errorf("container %s not found in LWS %s", containerName, lws.Name)
}

// ExpectContainerProbe implements WorkloadChecker
func (l *LWSChecker) ExpectContainerProbe(appName, roleName, containerName, probeType string, expected *corev1.Probe) error {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
		return err
	}

	for _, container := range lws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.Containers {
		if container.Name == containerName {
			var actual *corev1.Probe
			switch probeType {
			case "liveness":
				actual = container.LivenessProbe
			case "readiness":
				actual = container.ReadinessProbe
			case "startup":
				actual = container.StartupProbe
			default:
				return fmt.Errorf("unknown probe type: %s", probeType)
			}

			if !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("%s probe mismatch: expected %v, got %v",
					probeType, expected, actual)
			}
			return nil
		}
	}

	return fmt.Errorf("container %s not found in LWS %s", containerName, lws.Name)
}

// ExpectVolumeMounts implements WorkloadChecker
func (l *LWSChecker) ExpectVolumeMounts(appName, roleName string, expected []corev1.VolumeMount) error {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
		return err
	}

	// Get volume mounts from the first container
	if len(lws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.Containers) == 0 {
		return fmt.Errorf("no containers in LWS %s", lws.Name)
	}

	container := lws.Spec.LeaderWorkerTemplate.LeaderTemplate.Spec.Containers[0]

	for _, expectedMount := range expected {
		found := false
		for _, actualMount := range container.VolumeMounts {
			if actualMount.Name == expectedMount.Name &&
				actualMount.MountPath == expectedMount.MountPath {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("volume mount %s at %s not found",
				expectedMount.Name, expectedMount.MountPath)
		}
	}

	return nil
}

// ExpectLWSReplicas is a convenience method for verifying LWS replicas using gomega
func ExpectLWSReplicas(ctx context.Context, c client.Client, namespace, appName, roleName string, expected int32) {
	checker := NewLWSChecker(ctx, c, namespace)
	gomega.Eventually(func() error {
		return checker.ExpectWorkloadReady(appName, roleName, expected)
	}).Should(gomega.Succeed(), "LWS for %s/%s should have %d replicas", appName, roleName, expected)
}

// GetLWSPartition returns the partition value from the LWS rollout strategy
// Returns -1 if LWS not found or partition not set, 0 if partition is nil (default)
func (l *LWSChecker) GetLWSPartition(appName, roleName string) int32 {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
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

// GetLWSRolloutStrategy returns the full rollout strategy for inspection
func (l *LWSChecker) GetLWSRolloutStrategy(appName, roleName string) *lwsv1.RolloutStrategy {
	lws, err := l.getLWS(appName, roleName)
	if err != nil {
		return nil
	}
	return &lws.Spec.RolloutStrategy
}
