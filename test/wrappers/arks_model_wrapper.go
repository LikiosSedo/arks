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

package wrappers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

// ArksModelWrapper wraps ArksModel for builder pattern
type ArksModelWrapper struct {
	arksv1.ArksModel
}

// Obj returns the underlying ArksModel object
func (w *ArksModelWrapper) Obj() *arksv1.ArksModel {
	return &w.ArksModel
}

// WithName sets the name
func (w *ArksModelWrapper) WithName(name string) *ArksModelWrapper {
	w.Name = name
	return w
}

// WithNamespace sets the namespace
func (w *ArksModelWrapper) WithNamespace(ns string) *ArksModelWrapper {
	w.Namespace = ns
	return w
}

// WithModel sets the model path/name
func (w *ArksModelWrapper) WithModel(model string) *ArksModelWrapper {
	w.Spec.Model = model
	return w
}

// WithPVCName sets the PVC name for storage
func (w *ArksModelWrapper) WithPVCName(name string) *ArksModelWrapper {
	if w.Spec.Storage == nil {
		w.Spec.Storage = &arksv1.ArksModelStorage{}
	}
	if w.Spec.Storage.PVC == nil {
		w.Spec.Storage.PVC = &arksv1.ArksModelStoragePVC{}
	}
	w.Spec.Storage.PVC.Name = name
	return w
}

// WithPVCSpec sets the PVC spec for storage
func (w *ArksModelWrapper) WithPVCSpec(spec corev1.PersistentVolumeClaimSpec) *ArksModelWrapper {
	if w.Spec.Storage == nil {
		w.Spec.Storage = &arksv1.ArksModelStorage{}
	}
	if w.Spec.Storage.PVC == nil {
		w.Spec.Storage.PVC = &arksv1.ArksModelStoragePVC{}
	}
	w.Spec.Storage.PVC.Spec = spec
	return w
}

// WithStorageClassName sets the storage class name
func (w *ArksModelWrapper) WithStorageClassName(className string) *ArksModelWrapper {
	if w.Spec.Storage == nil {
		w.Spec.Storage = &arksv1.ArksModelStorage{}
	}
	if w.Spec.Storage.PVC == nil {
		w.Spec.Storage.PVC = &arksv1.ArksModelStoragePVC{}
	}
	w.Spec.Storage.PVC.Spec.StorageClassName = &className
	return w
}

// WithStorageSize sets the storage size
func (w *ArksModelWrapper) WithStorageSize(size string) *ArksModelWrapper {
	if w.Spec.Storage == nil {
		w.Spec.Storage = &arksv1.ArksModelStorage{}
	}
	if w.Spec.Storage.PVC == nil {
		w.Spec.Storage.PVC = &arksv1.ArksModelStoragePVC{}
	}
	if w.Spec.Storage.PVC.Spec.Resources.Requests == nil {
		w.Spec.Storage.PVC.Spec.Resources.Requests = corev1.ResourceList{}
	}
	w.Spec.Storage.PVC.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
	return w
}

// WithSubPath sets the subpath for storage
func (w *ArksModelWrapper) WithSubPath(subPath string) *ArksModelWrapper {
	if w.Spec.Storage == nil {
		w.Spec.Storage = &arksv1.ArksModelStorage{}
	}
	w.Spec.Storage.SubPath = subPath
	return w
}

// WithHuggingFaceSource sets the Hugging Face source
func (w *ArksModelWrapper) WithHuggingFaceSource(tokenSecretName string) *ArksModelWrapper {
	w.Spec.Source = &arksv1.ArksModelSource{
		Huggingface: &arksv1.ArksModelSourceHuggingFace{},
	}
	if tokenSecretName != "" {
		w.Spec.Source.Huggingface.TokenSecretRef = &corev1.LocalObjectReference{
			Name: tokenSecretName,
		}
	}
	return w
}

// WithLabels sets custom labels
func (w *ArksModelWrapper) WithLabels(labels map[string]string) *ArksModelWrapper {
	if w.Labels == nil {
		w.Labels = make(map[string]string)
	}
	for k, v := range labels {
		w.Labels[k] = v
	}
	return w
}

// WithAnnotations sets custom annotations
func (w *ArksModelWrapper) WithAnnotations(annotations map[string]string) *ArksModelWrapper {
	if w.Annotations == nil {
		w.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		w.Annotations[k] = v
	}
	return w
}

// BuildMockArksModel creates a mock ArksModel for testing
// This model references an existing PVC (test-model-pvc) created by the framework
func BuildMockArksModel(name, namespace string) *ArksModelWrapper {
	return &ArksModelWrapper{
		arksv1.ArksModel{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "arks.ai/v1",
				Kind:       "ArksModel",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: arksv1.ArksModelSpec{
				Model: "mock-model/test",
				Storage: &arksv1.ArksModelStorage{
					PVC: &arksv1.ArksModelStoragePVC{
						// Reference the PVC created by the test framework
						Name: "test-model-pvc",
					},
				},
			},
		},
	}
}

// BuildMockArksModelWithPVC creates a mock ArksModel referencing an existing PVC
func BuildMockArksModelWithPVC(name, namespace, pvcName string) *ArksModelWrapper {
	return &ArksModelWrapper{
		arksv1.ArksModel{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "arks.ai/v1",
				Kind:       "ArksModel",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: arksv1.ArksModelSpec{
				Model: "mock-model/test",
				Storage: &arksv1.ArksModelStorage{
					PVC: &arksv1.ArksModelStoragePVC{
						Name: pvcName,
					},
				},
			},
		},
	}
}

// BuildRealArksModel creates an ArksModel for real GPU cluster testing
// This should reference an actual model on a real PVC
func BuildRealArksModel(name, namespace string) *ArksModelWrapper {
	return &ArksModelWrapper{
		arksv1.ArksModel{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "arks.ai/v1",
				Kind:       "ArksModel",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: arksv1.ArksModelSpec{
				Source: &arksv1.ArksModelSource{
					Huggingface: &arksv1.ArksModelSourceHuggingFace{},
				},
			},
		},
	}
}

// WithPVCStorage sets PVC storage with full configuration
func (w *ArksModelWrapper) WithPVCStorage(pvcName, storageClass, size string) *ArksModelWrapper {
	if w.Spec.Storage == nil {
		w.Spec.Storage = &arksv1.ArksModelStorage{}
	}
	w.Spec.Storage.PVC = &arksv1.ArksModelStoragePVC{
		Name: pvcName,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}
	return w
}
