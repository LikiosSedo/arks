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
	"context"
	"fmt"
	"time"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
	rbgv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

const (
	// Timeout is the default timeout for Eventually assertions
	// Extended for real cluster operations
	Timeout = 120 * time.Second
	// ShortTimeout is a shorter timeout for quick checks
	ShortTimeout = 10 * time.Second
	// GPUTimeout is the timeout for GPU workloads which need longer startup time
	// sglang model loading can take 3-5 minutes
	GPUTimeout = 300 * time.Second
	// Interval is the polling interval for Eventually assertions
	Interval = 500 * time.Millisecond

	// DefaultMockImage is the default image for testing without GPU (using siflow registry)
	// Using doks-debug which stays running (unlike busybox which exits immediately)
	DefaultMockImage = "registry-cn-shanghai.siflow.cn/k8s/digitalocean/doks-debug:latest"
	// MockRouterImage is the mock image for router component
	MockRouterImage = "registry-cn-shanghai.siflow.cn/k8s/digitalocean/doks-debug:latest"
	// MockRuntimeImage is the mock image for runtime component
	MockRuntimeImage = "registry-cn-shanghai.siflow.cn/k8s/digitalocean/doks-debug:latest"
)

const (
	// TestModelPVCName is the name of the PVC used for test models
	TestModelPVCName = "test-model-pvc"
)

// Framework provides a testing framework for e2e tests
type Framework struct {
	Ctx              context.Context
	Client           client.Client
	Namespace        string
	scheme           *runtime.Scheme
	pvName           string // Name of the PV created for this test suite
	SkipModelCleanup bool   // Skip ArksModel cleanup in AfterEach (for tests using pre-existing models)
}

// NewFramework creates a new testing framework that connects to the current kubeconfig cluster
func NewFramework() *Framework {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(arksv1.AddToScheme(scheme))
	utilruntime.Must(rbgv1alpha1.AddToScheme(scheme))
	utilruntime.Must(lwsv1.AddToScheme(scheme))

	cfg, err := config.GetConfig()
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "Failed to get kubeconfig")

	runtimeClient, err := client.New(cfg, client.Options{Scheme: scheme})
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), "Failed to create controller-runtime client")

	return &Framework{
		Ctx:    context.Background(),
		Client: runtimeClient,
		scheme: scheme,
	}
}

// BeforeAll should be called in Ginkgo BeforeAll to set up the test namespace
func (f *Framework) BeforeAll() {
	logger := log.FromContext(f.Ctx)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "arks-e2e-",
			Labels: map[string]string{
				"arks-e2e-test": "true",
			},
		},
	}
	err := f.Client.Create(f.Ctx, ns)
	gomega.Expect(err).Should(gomega.Succeed(), "Failed to create test namespace")

	f.Namespace = ns.Name
	logger.Info("Created test namespace", "namespace", f.Namespace)

	// Wait for namespace to be active
	gomega.Eventually(func() bool {
		fetchedNs := &corev1.Namespace{}
		if err := f.Client.Get(f.Ctx, client.ObjectKey{Name: f.Namespace}, fetchedNs); err != nil {
			return false
		}
		return fetchedNs.Status.Phase == corev1.NamespaceActive
	}, Timeout, Interval).Should(gomega.BeTrue(), "Namespace should become active")

	// Create PV and PVC for test models
	f.createTestModelStorage()
}

// createTestModelStorage creates a hostPath PV and corresponding PVC for test models
func (f *Framework) createTestModelStorage() {
	logger := log.FromContext(f.Ctx)

	// Create a unique PV name for this test suite
	f.pvName = fmt.Sprintf("arks-e2e-pv-%s", f.Namespace)

	// Create hostPath PV
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.pvName,
			Labels: map[string]string{
				"arks-e2e-test": "true",
				"namespace":     f.Namespace,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: fmt.Sprintf("/tmp/arks-e2e/%s", f.Namespace),
				},
			},
			// Bind to the specific PVC in test namespace
			ClaimRef: &corev1.ObjectReference{
				Namespace: f.Namespace,
				Name:      TestModelPVCName,
			},
		},
	}

	err := f.Client.Create(f.Ctx, pv)
	gomega.Expect(err).Should(gomega.Succeed(), "Failed to create test PV")
	logger.Info("Created test PV", "pv", f.pvName)

	// Create PVC in test namespace
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestModelPVCName,
			Namespace: f.Namespace,
			Labels: map[string]string{
				"arks-e2e-test": "true",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			VolumeName: f.pvName,
		},
	}

	err = f.Client.Create(f.Ctx, pvc)
	gomega.Expect(err).Should(gomega.Succeed(), "Failed to create test PVC")
	logger.Info("Created test PVC", "pvc", TestModelPVCName)

	// Wait for PVC to be bound
	gomega.Eventually(func() corev1.PersistentVolumeClaimPhase {
		fetchedPVC := &corev1.PersistentVolumeClaim{}
		if err := f.Client.Get(f.Ctx, client.ObjectKey{Namespace: f.Namespace, Name: TestModelPVCName}, fetchedPVC); err != nil {
			return ""
		}
		return fetchedPVC.Status.Phase
	}, Timeout, Interval).Should(gomega.Equal(corev1.ClaimBound), "PVC should be bound")
}

// AfterAll should be called in Ginkgo AfterAll to clean up the test namespace
func (f *Framework) AfterAll() {
	logger := log.FromContext(f.Ctx)

	if f.Namespace == "" {
		return
	}

	// Delete namespace (this will cascade delete PVC)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.Namespace,
		},
	}
	err := f.Client.Delete(f.Ctx, ns)
	if err != nil {
		logger.Error(err, "Failed to delete test namespace", "namespace", f.Namespace)
	} else {
		logger.Info("Deleted test namespace", "namespace", f.Namespace)
	}

	// Delete PV (PV is cluster-scoped, not deleted with namespace)
	if f.pvName != "" {
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.pvName,
			},
		}
		err = f.Client.Delete(f.Ctx, pv)
		if err != nil {
			logger.Error(err, "Failed to delete test PV", "pv", f.pvName)
		} else {
			logger.Info("Deleted test PV", "pv", f.pvName)
		}
	}
}

// AfterEach should be called in Ginkgo AfterEach to clean up test resources
func (f *Framework) AfterEach() {
	logger := log.FromContext(f.Ctx)

	// Clean up all ArksDisaggregatedApplications in the test namespace
	err := f.Client.DeleteAllOf(
		f.Ctx,
		&arksv1.ArksDisaggregatedApplication{},
		client.InNamespace(f.Namespace),
	)
	if err != nil {
		logger.Error(err, "Failed to delete ArksDisaggregatedApplications")
	}

	// Clean up all ArksApplications in the test namespace
	err = f.Client.DeleteAllOf(
		f.Ctx,
		&arksv1.ArksApplication{},
		client.InNamespace(f.Namespace),
	)
	if err != nil {
		logger.Error(err, "Failed to delete ArksApplications")
	}

	// Clean up all ArksModels in the test namespace (unless SkipModelCleanup is set)
	if !f.SkipModelCleanup {
		err = f.Client.DeleteAllOf(
			f.Ctx,
			&arksv1.ArksModel{},
			client.InNamespace(f.Namespace),
		)
		if err != nil {
			logger.Error(err, "Failed to delete ArksModels")
		}
	} else {
		logger.Info("Skipping ArksModel cleanup (SkipModelCleanup=true)")
	}

	// Wait for cleanup to complete
	gomega.Eventually(func() int {
		list := &arksv1.ArksDisaggregatedApplicationList{}
		if err := f.Client.List(f.Ctx, list, client.InNamespace(f.Namespace)); err != nil {
			return -1
		}
		return len(list.Items)
	}, Timeout, Interval).Should(gomega.Equal(0), "All ArksDisaggregatedApplications should be deleted")

	// Wait for ArksModels cleanup to complete (unless SkipModelCleanup is set)
	if !f.SkipModelCleanup {
		gomega.Eventually(func() int {
			list := &arksv1.ArksModelList{}
			if err := f.Client.List(f.Ctx, list, client.InNamespace(f.Namespace)); err != nil {
				return -1
			}
			return len(list.Items)
		}, Timeout, Interval).Should(gomega.Equal(0), "All ArksModels should be deleted")
	}

	gomega.Eventually(func() int {
		list := &arksv1.ArksApplicationList{}
		if err := f.Client.List(f.Ctx, list, client.InNamespace(f.Namespace)); err != nil {
			return -1
		}
		return len(list.Items)
	}, Timeout, Interval).Should(gomega.Equal(0), "All ArksApplications should be deleted")
}

// GetScheme returns the scheme used by the framework
func (f *Framework) GetScheme() *runtime.Scheme {
	return f.scheme
}
