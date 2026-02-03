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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	arksv1 "github.com/arks-ai/arks/api/v1"
	"github.com/arks-ai/arks/test/e2e/framework"
	"github.com/arks-ai/arks/test/utils"
	"github.com/arks-ai/arks/test/wrappers"
)

// RunDisaggVolumeTestCases runs volume mount tests
func RunDisaggVolumeTestCases(fp **framework.Framework) {
	ginkgo.Describe("ArksDisaggregatedApplication Volumes", ginkgo.Label("volume"), func() {

		ginkgo.BeforeEach(func() {
			f := *fp
			model := wrappers.BuildMockArksModel("test-model", f.Namespace).Obj()
			gomega.Expect(f.Client.Create(f.Ctx, model)).Should(gomega.Succeed())
			f.ExpectArksModelReady(model)
		})

		ginkgo.It("should add emptyDir volume to prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-vol-prefill-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add volume
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.Volumes = []corev1.Volume{
					{
						Name: "cache-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
				a.Spec.Prefill.InstanceSpec.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "cache-volume",
						MountPath: "/cache",
					},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should update volume mount path for prefill", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-vol-prefill-update", f.Namespace).
				WithPrefillVolumes([]corev1.Volume{
					{
						Name: "cache-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}).
				WithPrefillVolumeMounts([]corev1.VolumeMount{
					{
						Name:      "cache-volume",
						MountPath: "/cache",
					},
				}).
				Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Update mount path
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "cache-volume",
						MountPath: "/data/cache",
					},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add emptyDir volume to decode", func() {
			f := *fp
			app := wrappers.BuildBasicArksDisaggApp("e2e-vol-decode-add", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add volume
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.Volumes = []corev1.Volume{
					{
						Name: "decode-cache",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
				a.Spec.Decode.InstanceSpec.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "decode-cache",
						MountPath: "/decode-cache",
					},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add configMap volume to prefill", func() {
			f := *fp
			// First create a ConfigMap
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: f.Namespace,
				},
				Data: map[string]string{
					"config.yaml": "key: value",
				},
			}
			gomega.Expect(f.Client.Create(f.Ctx, cm)).Should(gomega.Succeed())

			app := wrappers.BuildBasicArksDisaggApp("e2e-vol-configmap", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add configMap volume
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Prefill.InstanceSpec.Volumes = []corev1.Volume{
					{
						Name: "config-volume",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "test-config",
								},
							},
						},
					},
				}
				a.Spec.Prefill.InstanceSpec.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "config-volume",
						MountPath: "/config",
					},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})

		ginkgo.It("should add secret volume to decode", func() {
			f := *fp
			// First create a Secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: f.Namespace,
				},
				StringData: map[string]string{
					"token": "secret-token",
				},
			}
			gomega.Expect(f.Client.Create(f.Ctx, secret)).Should(gomega.Succeed())

			app := wrappers.BuildBasicArksDisaggApp("e2e-vol-secret", f.Namespace).Obj()

			gomega.Expect(f.Client.Create(f.Ctx, app)).Should(gomega.Succeed())
			f.ExpectArksDisaggAppReady(app)

			// Add secret volume
			utils.UpdateArksDisaggApp(f.Ctx, f.Client, app, func(a *arksv1.ArksDisaggregatedApplication) {
				a.Spec.Decode.InstanceSpec.Volumes = []corev1.Volume{
					{
						Name: "secret-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "test-secret",
							},
						},
					},
				}
				a.Spec.Decode.InstanceSpec.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "secret-volume",
						MountPath: "/secrets",
					},
				}
			})

			f.ExpectArksDisaggAppReady(app)
			f.ExpectRollingUpdateComplete(app)
		})
	})
}
