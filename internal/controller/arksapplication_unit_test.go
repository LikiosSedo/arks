package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

func TestGenerateRBGSUnifiedWithoutRouter(t *testing.T) {
	reconciler := &ArksApplicationReconciler{}
	application := newTestArksApplication()
	model := newTestArksModel()

	rbgs, err := reconciler.generateRBGS(context.Background(), application, model)
	if err != nil {
		t.Fatalf("generateRBGS returned error: %v", err)
	}

	if len(rbgs.Spec.Template.Roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(rbgs.Spec.Template.Roles))
	}
	if rbgs.Spec.Template.Roles[0].Name != "unified" {
		t.Fatalf("expected unified role, got %s", rbgs.Spec.Template.Roles[0].Name)
	}
	if len(rbgs.Spec.Template.CoordinationRequirements) != 0 {
		t.Fatalf("expected no coordination requirements, got %d", len(rbgs.Spec.Template.CoordinationRequirements))
	}
}

func TestGenerateRBGSUnifiedWithRouter(t *testing.T) {
	reconciler := &ArksApplicationReconciler{}
	application := newTestArksApplication()
	application.Spec.Runtime = string(arksv1.ArksRuntimeSGLang)
	application.Spec.Router = &arksv1.ArksApplicationRouter{
		InstanceSpec: arksv1.ArksInstanceSpec{ServiceAccountName: "router-sa"},
	}
	model := newTestArksModel()

	rbgs, err := reconciler.generateRBGS(context.Background(), application, model)
	if err != nil {
		t.Fatalf("generateRBGS returned error: %v", err)
	}

	if len(rbgs.Spec.Template.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(rbgs.Spec.Template.Roles))
	}
	if rbgs.Spec.Template.Roles[0].Name != "unified" {
		t.Fatalf("expected first role unified, got %s", rbgs.Spec.Template.Roles[0].Name)
	}
	if rbgs.Spec.Template.Roles[1].Name != "router" {
		t.Fatalf("expected second role router, got %s", rbgs.Spec.Template.Roles[1].Name)
	}
}

func TestGenerateRBGSDisaggregatedWithCoordination(t *testing.T) {
	reconciler := &ArksApplicationReconciler{}
	application := newTestArksApplication()
	application.Spec.Mode = arksv1.ArksApplicationModeDisaggregated
	application.Spec.Runtime = string(arksv1.ArksRuntimeSGLang)
	application.Spec.Unified = nil
	application.Spec.Router = &arksv1.ArksApplicationRouter{
		InstanceSpec: arksv1.ArksInstanceSpec{ServiceAccountName: "router-sa"},
	}
	application.Spec.Prefill = &arksv1.ArksApplicationWorkload{
		Replicas: ptrInt32(2),
		Size:     1,
	}
	application.Spec.Decode = &arksv1.ArksApplicationWorkload{
		Replicas: ptrInt32(4),
		Size:     1,
	}
	application.Spec.CoordinationPolicy = &arksv1.CoordinationPolicy{
		Scaling: &arksv1.ScalingCoordination{
			MaxSkew:     "10%",
			Progression: "OrderReady",
		},
	}
	model := newTestArksModel()

	rbgs, err := reconciler.generateRBGS(context.Background(), application, model)
	if err != nil {
		t.Fatalf("generateRBGS returned error: %v", err)
	}

	if len(rbgs.Spec.Template.Roles) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(rbgs.Spec.Template.Roles))
	}
	if rbgs.Spec.Template.Roles[0].Name != "router" || rbgs.Spec.Template.Roles[1].Name != "prefill" || rbgs.Spec.Template.Roles[2].Name != "decode" {
		t.Fatalf("unexpected role order: %s, %s, %s", rbgs.Spec.Template.Roles[0].Name, rbgs.Spec.Template.Roles[1].Name, rbgs.Spec.Template.Roles[2].Name)
	}
	if len(rbgs.Spec.Template.CoordinationRequirements) == 0 {
		t.Fatal("expected coordination requirements for disaggregated mode")
	}
}

func TestIsArksApplicationReady(t *testing.T) {
	tests := []struct {
		name string
		app  *arksv1.ArksApplication
		want bool
	}{
		{
			name: "unified without router ready",
			app: &arksv1.ArksApplication{
				Status: arksv1.ArksApplicationStatus{
					Unified: arksv1.ArksRoleStatus{Replicas: 2, ReadyReplicas: 2},
				},
			},
			want: true,
		},
		{
			name: "unified with router ready",
			app: &arksv1.ArksApplication{
				Spec: arksv1.ArksApplicationSpec{
					Router: &arksv1.ArksApplicationRouter{},
				},
				Status: arksv1.ArksApplicationStatus{
					Mode:          arksv1.ArksApplicationModeUnified,
					TrafficTarget: arksv1.ArksApplicationTrafficTargetRouter,
					Unified:     arksv1.ArksRoleStatus{Replicas: 1, ReadyReplicas: 1},
					Router:        arksv1.ArksRoleStatus{Replicas: 1, ReadyReplicas: 1},
					Conditions: []arksv1.ArksApplicationCondition{
						{Type: arksv1.ArksApplicationTrafficTargetReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		{
			name: "disaggregated ready",
			app: &arksv1.ArksApplication{
				Status: arksv1.ArksApplicationStatus{
					Mode:   arksv1.ArksApplicationModeDisaggregated,
					Router: arksv1.ArksRoleStatus{Replicas: 1, ReadyReplicas: 1},
					Prefill: arksv1.ArksRoleStatus{
						Replicas: 2, ReadyReplicas: 2,
					},
					Decode: arksv1.ArksRoleStatus{
						Replicas: 4, ReadyReplicas: 4,
					},
					Conditions: []arksv1.ArksApplicationCondition{
						{Type: arksv1.ArksApplicationTrafficTargetReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		{
			name: "pending traffic target is not ready",
			app: &arksv1.ArksApplication{
				Status: arksv1.ArksApplicationStatus{
					Mode:          arksv1.ArksApplicationModeUnified,
					TrafficTarget: arksv1.ArksApplicationTrafficTargetPending,
					Unified:     arksv1.ArksRoleStatus{Replicas: 1, ReadyReplicas: 1},
				},
			},
			want: false,
		},
		{
			name: "router target not ready",
			app: &arksv1.ArksApplication{
				Spec: arksv1.ArksApplicationSpec{
					Router: &arksv1.ArksApplicationRouter{},
				},
				Status: arksv1.ArksApplicationStatus{
					Mode:      arksv1.ArksApplicationModeUnified,
					Unified: arksv1.ArksRoleStatus{Replicas: 1, ReadyReplicas: 1},
					Router:    arksv1.ArksRoleStatus{Replicas: 1, ReadyReplicas: 0},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isArksApplicationReady(tt.app); got != tt.want {
				t.Fatalf("isArksApplicationReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureApplicationServiceUsesStableServicePort(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := arksv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add arksv1 to scheme: %v", err)
	}

	application := newTestArksApplication()
	application.Name = "test-app"
	application.Namespace = "default"
	application.Spec.Runtime = string(arksv1.ArksRuntimeSGLang)
	application.Spec.Router = &arksv1.ArksApplicationRouter{Port: 31000}

	reconciler := &ArksApplicationReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(application).
			Build(),
		Scheme: scheme,
	}

	if err := reconciler.ensureApplicationService(context.Background(), application, arksv1.ArksApplicationTrafficTargetRouter); err != nil {
		t.Fatalf("ensureApplicationService returned error: %v", err)
	}

	service := &corev1.Service{}
	if err := reconciler.Client.Get(context.Background(), types.NamespacedName{
		Namespace: application.Namespace,
		Name:      generateApplicationServiceName(application),
	}, service); err != nil {
		t.Fatalf("failed to get created service: %v", err)
	}

	if len(service.Spec.Ports) != 1 {
		t.Fatalf("expected 1 service port, got %d", len(service.Spec.Ports))
	}
	if service.Spec.Ports[0].Port != 8080 {
		t.Fatalf("expected stable service port 8080, got %d", service.Spec.Ports[0].Port)
	}
	if service.Spec.Ports[0].TargetPort != intstr.FromInt(31000) {
		t.Fatalf("expected target port 31000, got %+v", service.Spec.Ports[0].TargetPort)
	}
}

func TestSyncApplicationAggregateStatus(t *testing.T) {
	tests := []struct {
		name string
		app  *arksv1.ArksApplication
		want arksv1.ArksApplicationStatus
	}{
		{
			name: "unified aggregates inference only",
			app: &arksv1.ArksApplication{
				Spec: arksv1.ArksApplicationSpec{
					Mode: arksv1.ArksApplicationModeUnified,
				},
				Status: arksv1.ArksApplicationStatus{
					Unified: arksv1.ArksRoleStatus{
						Replicas:        2,
						ReadyReplicas:   2,
						UpdatedReplicas: 1,
					},
					Router: arksv1.ArksRoleStatus{
						Replicas:        1,
						ReadyReplicas:   1,
						UpdatedReplicas: 1,
					},
				},
			},
			want: arksv1.ArksApplicationStatus{
				Replicas:        2,
				ReadyReplicas:   2,
				UpdatedReplicas: 1,
			},
		},
		{
			name: "disaggregated aggregates active roles",
			app: &arksv1.ArksApplication{
				Spec: arksv1.ArksApplicationSpec{
					Mode: arksv1.ArksApplicationModeDisaggregated,
				},
				Status: arksv1.ArksApplicationStatus{
					Router: arksv1.ArksRoleStatus{
						Replicas:        1,
						ReadyReplicas:   1,
						UpdatedReplicas: 1,
					},
					Prefill: arksv1.ArksRoleStatus{
						Replicas:        2,
						ReadyReplicas:   2,
						UpdatedReplicas: 2,
					},
					Decode: arksv1.ArksRoleStatus{
						Replicas:        4,
						ReadyReplicas:   3,
						UpdatedReplicas: 2,
					},
				},
			},
			want: arksv1.ArksApplicationStatus{
				Replicas:        7,
				ReadyReplicas:   6,
				UpdatedReplicas: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncApplicationAggregateStatus(tt.app)
			if tt.app.Status.Replicas != tt.want.Replicas {
				t.Fatalf("Replicas = %d, want %d", tt.app.Status.Replicas, tt.want.Replicas)
			}
			if tt.app.Status.ReadyReplicas != tt.want.ReadyReplicas {
				t.Fatalf("ReadyReplicas = %d, want %d", tt.app.Status.ReadyReplicas, tt.want.ReadyReplicas)
			}
			if tt.app.Status.UpdatedReplicas != tt.want.UpdatedReplicas {
				t.Fatalf("UpdatedReplicas = %d, want %d", tt.app.Status.UpdatedReplicas, tt.want.UpdatedReplicas)
			}
		})
	}
}

func newTestArksApplication() *arksv1.ArksApplication {
	return &arksv1.ArksApplication{
		Spec: arksv1.ArksApplicationSpec{
			Runtime: string(arksv1.ArksRuntimeVLLM),
			Model:   corev1.LocalObjectReference{Name: "test-model"},
			Unified: &arksv1.ArksApplicationWorkload{
				Replicas: ptrInt32(2),
				Size:     1,
				RuntimeCommonArgs: []string{
					"--tensor-parallel-size", "1",
				},
			},
		},
	}
}

func newTestArksModel() *arksv1.ArksModel {
	return &arksv1.ArksModel{
		Spec: arksv1.ArksModelSpec{
			Model: "test-model",
			Storage: &arksv1.ArksModelStorage{
				PVC: &arksv1.ArksModelStoragePVC{Name: "test-pvc"},
			},
		},
	}
}

func ptrInt32(v int32) *int32 {
	return &v
}
