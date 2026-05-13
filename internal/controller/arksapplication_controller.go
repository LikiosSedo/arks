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

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	lwscli "sigs.k8s.io/lws/client-go/clientset/versioned"
	rbgv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"

	arksv1 "github.com/arks-ai/arks/api/v1"
)

const (
	arksApplicationControllerFinalizer  = "application.arks.ai/controller"
	arksApplicationModelVolumeName      = "models"
	arksApplicationModelVolumeMountPath = "/models"
)

// ArksApplicationReconciler reconciles an ArksApplication object.
type ArksApplicationReconciler struct {
	client.Client
	KubeClient *kubernetes.Clientset
	LWSClient  *lwscli.Clientset
	Scheme     *runtime.Scheme
}

const arksApplicationModelField = "spec.model.name"

// +kubebuilder:rbac:groups=arks.ai,resources=arksapplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=arks.ai,resources=arksapplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=arks.ai,resources=arksapplications/finalizers,verbs=update
// +kubebuilder:rbac:groups=workloads.x-k8s.io,resources=rolebasedgroupsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workloads.x-k8s.io,resources=rolebasedgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *ArksApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	application := &arksv1.ArksApplication{}
	if err := r.Client.Get(ctx, req.NamespacedName, application, &client.GetOptions{
		Raw: &metav1.GetOptions{ResourceVersion: ""},
	}); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if application.DeletionTimestamp != nil {
		klog.Infof("application %s/%s: remove application", application.Namespace, application.Name)
		return r.remove(ctx, application)
	}

	original := application.DeepCopy()
	result, err := r.reconcile(ctx, application)

	if statusErr := r.patchApplicationStatus(ctx, original, application); statusErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status for application %s/%s (%s): %w", application.Namespace, application.Name, application.UID, statusErr)
	}

	if err != nil {
		klog.Errorf("failed to reconcile application %s/%s (%s): %q", application.Namespace, application.Name, application.UID, err)
		return result, err
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArksApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()

	if err := mgr.GetFieldIndexer().IndexField(ctx, &arksv1.ArksApplication{}, arksApplicationModelField, func(obj client.Object) []string {
		app, ok := obj.(*arksv1.ArksApplication)
		if !ok || app.Spec.Model.Name == "" {
			return nil
		}
		return []string{app.Spec.Model.Name}
	}); err != nil {
		return fmt.Errorf("failed to index arksapplication by model: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 30}).
		For(&arksv1.ArksApplication{}).
		Named("arksapplication").
		Owns(&rbgv1alpha1.RoleBasedGroupSet{}).
		Watches(&arksv1.ArksModel{}, handler.EnqueueRequestsFromMapFunc(r.requestsForModel)).
		Watches(&rbgv1alpha1.RoleBasedGroup{}, handler.EnqueueRequestsFromMapFunc(r.requestsForRBG)).
		Complete(r)
}

func (r *ArksApplicationReconciler) remove(ctx context.Context, application *arksv1.ArksApplication) (ctrl.Result, error) {
	if application.DeletionTimestamp == nil {
		return ctrl.Result{Requeue: true}, nil
	}

	serviceName := generateApplicationServiceName(application)
	if err := r.KubeClient.CoreV1().Services(application.Namespace).Delete(ctx, serviceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to delete application service (%s): %q", serviceName, err)
	}

	rbgs := &rbgv1alpha1.RoleBasedGroupSet{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: application.Namespace, Name: application.Name}, rbgs); err == nil {
		if err := r.Client.Delete(ctx, rbgs); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to delete underlying RBGS: %q", err)
		}
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to check RBGS: %q", err)
	}

	// Delete any legacy LWS with the old standalone name for cleanup compatibility.
	if r.LWSClient != nil {
		if err := r.LWSClient.LeaderworkersetV1().LeaderWorkerSets(application.Namespace).Delete(ctx, application.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to delete legacy LWS: %q", err)
		}
	}

	if err := r.removeFinalizerWithRetry(ctx, application); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove application finalizer: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *ArksApplicationReconciler) reconcile(ctx context.Context, application *arksv1.ArksApplication) (ctrl.Result, error) {
	if application.DeletionTimestamp != nil {
		return ctrl.Result{Requeue: true}, nil
	}

	if application.Status.Phase == "" {
		application.Status.Phase = string(arksv1.ArksApplicationPhasePending)
	}

	initializeApplicationCondition(application)

	if !hasFinalizer(application, arksApplicationControllerFinalizer) {
		addFinalizer(application, arksApplicationControllerFinalizer)
		if err := r.Client.Update(ctx, application); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add application finalizer: %q", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.validate(application); err != nil {
		application.Status.Phase = string(arksv1.ArksApplicationPhaseFailed)
		updateApplicationCondition(application, arksv1.ArksApplicationPrecheck, corev1.ConditionFalse, "ValidationFailed", err.Error())
		updateApplicationCondition(application, arksv1.ArksApplicationReady, corev1.ConditionFalse, "ValidationFailed", err.Error())
		return ctrl.Result{}, nil
	}

	if !checkApplicationCondition(application, arksv1.ArksApplicationPrecheck) {
		application.Status.Phase = string(arksv1.ArksApplicationPhaseChecking)
		updateApplicationCondition(application, arksv1.ArksApplicationPrecheck, corev1.ConditionTrue, "PrecheckPass", "The application passed the pre-checking")
	}

	model := &arksv1.ArksModel{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: application.Namespace, Name: application.Spec.Model.Name}, model, &client.GetOptions{
		Raw: &metav1.GetOptions{ResourceVersion: ""},
	}); err != nil {
		if apierrors.IsNotFound(err) {
			application.Status.Phase = string(arksv1.ArksApplicationPhaseFailed)
			updateApplicationCondition(application, arksv1.ArksApplicationLoaded, corev1.ConditionFalse, "ModelNotExist", "The referenced model doesn't exist")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !checkApplicationCondition(application, arksv1.ArksApplicationLoaded) {
		application.Status.Phase = string(arksv1.ArksApplicationPhaseLoading)
		switch model.Status.Phase {
		case string(arksv1.ArksModelPhaseFailed):
			application.Status.Phase = string(arksv1.ArksApplicationPhaseFailed)
			updateApplicationCondition(application, arksv1.ArksApplicationLoaded, corev1.ConditionFalse, "ModelLoadFailed", "Failed to load the referenced model")
			return ctrl.Result{}, nil
		case string(arksv1.ArksModelReady):
			updateApplicationCondition(application, arksv1.ArksApplicationLoaded, corev1.ConditionTrue, "ModelLoadSucceeded", "The referenced model is loaded")
		default:
			return ctrl.Result{}, nil
		}
	}

	if err := r.reconcileRBGS(ctx, application, model); err != nil {
		application.Status.Phase = string(arksv1.ArksApplicationPhaseFailed)
		updateApplicationCondition(application, arksv1.ArksApplicationReady, corev1.ConditionFalse, "UnderlayReconcileFailed", fmt.Sprintf("Failed to reconcile RBGS: %v", err))
		return ctrl.Result{}, fmt.Errorf("failed to reconcile RBGS: %w", err)
	}

	if err := r.syncApplicationStatus(ctx, application); err != nil {
		klog.Warningf("application %s/%s: failed to sync status: %v", application.Namespace, application.Name, err)
	}

	result, err := r.reconcileApplicationService(ctx, application)
	if err != nil {
		return result, err
	}

	r.updateApplicationPhase(application)
	return result, nil
}

func (r *ArksApplicationReconciler) validate(application *arksv1.ArksApplication) error {
	switch getApplicationMode(application) {
	case arksv1.ArksApplicationModeUnified:
		if application.Spec.Unified == nil {
			return fmt.Errorf("unified spec is required when mode=unified")
		}
	case arksv1.ArksApplicationModeDisaggregated:
		if application.Spec.Prefill == nil || application.Spec.Decode == nil {
			return fmt.Errorf("prefill and decode specs are required when mode=disaggregated")
		}
		if application.Spec.Router == nil {
			return fmt.Errorf("router spec is required when mode=disaggregated")
		}
	default:
		return fmt.Errorf("unsupported mode: %s", getApplicationMode(application))
	}

	switch getArksApplicationRuntime(application) {
	case string(arksv1.ArksRuntimeVLLM), string(arksv1.ArksRuntimeSGLang), string(arksv1.ArksRuntimeDynamo):
	default:
		return fmt.Errorf("runtime not supported: %s", getArksApplicationRuntime(application))
	}

	if isApplicationRouterRequested(application) && getArksApplicationRuntime(application) != string(arksv1.ArksRuntimeSGLang) {
		if !hasCustomRouter(application) {
			return fmt.Errorf("router with runtime=%s requires both spec.routerImage and spec.router.commandOverride", getArksApplicationRuntime(application))
		}
	}

	for _, instanceSpec := range r.instanceSpecsForValidation(application) {
		if err := validateReservedModelPaths(instanceSpec); err != nil {
			return err
		}
	}

	return nil
}

func (r *ArksApplicationReconciler) instanceSpecsForValidation(application *arksv1.ArksApplication) []*arksv1.ArksInstanceSpec {
	var specs []*arksv1.ArksInstanceSpec
	if application.Spec.Unified != nil {
		specs = append(specs, &application.Spec.Unified.InstanceSpec)
	}
	if application.Spec.Router != nil {
		specs = append(specs, &application.Spec.Router.InstanceSpec)
	}
	if application.Spec.Prefill != nil {
		specs = append(specs, &application.Spec.Prefill.InstanceSpec)
	}
	if application.Spec.Decode != nil {
		specs = append(specs, &application.Spec.Decode.InstanceSpec)
	}
	return specs
}

func validateReservedModelPaths(instanceSpec *arksv1.ArksInstanceSpec) error {
	for _, volume := range instanceSpec.Volumes {
		if volume.Name == arksApplicationModelVolumeName {
			return fmt.Errorf("volume name %q is reserved for ArksModel", arksApplicationModelVolumeName)
		}
	}
	for _, volumeMount := range instanceSpec.VolumeMounts {
		if volumeMount.MountPath == arksApplicationModelVolumeMountPath {
			return fmt.Errorf("volume mount path %q is reserved for ArksModel", arksApplicationModelVolumeMountPath)
		}
	}
	return nil
}

func (r *ArksApplicationReconciler) reconcileRBGS(ctx context.Context, application *arksv1.ArksApplication, model *arksv1.ArksModel) error {
	rbgs := &rbgv1alpha1.RoleBasedGroupSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      application.Name,
			Namespace: application.Namespace,
		},
	}

	result, err := controllerutil.CreateOrPatch(ctx, r.Client, rbgs, func() error {
		desired, err := r.generateRBGS(ctx, application, model)
		if err != nil {
			return err
		}
		if !rbgsSpecSemanticallyEqual(rbgs.Spec, desired.Spec) {
			rbgs.Spec = desired.Spec
		}
		rbgs.Labels = desired.Labels
		return controllerutil.SetControllerReference(application, rbgs, r.Scheme)
	})
	if err != nil {
		return err
	}

	if result != controllerutil.OperationResultNone {
		klog.Infof("application %s/%s: %s RBGS successfully", application.Namespace, application.Name, result)
	}
	return nil
}

func (r *ArksApplicationReconciler) generateRBGS(ctx context.Context, application *arksv1.ArksApplication, model *arksv1.ArksModel) (*rbgv1alpha1.RoleBasedGroupSet, error) {
	var roles []rbgv1alpha1.RoleSpec

	switch getApplicationMode(application) {
	case arksv1.ArksApplicationModeUnified:
		unifiedRole, err := r.buildUnifiedRole(application, model)
		if err != nil {
			return nil, fmt.Errorf("failed to build unified role: %w", err)
		}
		roles = append(roles, unifiedRole)
		if r.shouldRenderRouterRole(application) {
			routerRole, err := r.buildRouterRole(ctx, application)
			if err != nil {
				return nil, fmt.Errorf("failed to build router role: %w", err)
			}
			roles = append(roles, routerRole)
		}
	case arksv1.ArksApplicationModeDisaggregated:
		routerRole, err := r.buildRouterRole(ctx, application)
		if err != nil {
			return nil, fmt.Errorf("failed to build router role: %w", err)
		}
		prefillRole, err := r.buildDisaggregatedRole(application, model, "prefill")
		if err != nil {
			return nil, fmt.Errorf("failed to build prefill role: %w", err)
		}
		decodeRole, err := r.buildDisaggregatedRole(application, model, "decode")
		if err != nil {
			return nil, fmt.Errorf("failed to build decode role: %w", err)
		}
		roles = append(roles, routerRole, prefillRole, decodeRole)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", getApplicationMode(application))
	}

	rbgsReplicas := int32(1)
	rbgs := &rbgv1alpha1.RoleBasedGroupSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: application.Namespace,
			Name:      application.Name,
			Labels: map[string]string{
				arksv1.ArksControllerKeyApplication: application.Name,
				arksv1.ArksControllerKeyModel:       application.Spec.Model.Name,
			},
		},
		Spec: rbgv1alpha1.RoleBasedGroupSetSpec{
			Replicas: &rbgsReplicas,
			Template: rbgv1alpha1.RoleBasedGroupSpec{
				PodGroupPolicy: convertArksAppPodGroupPolicy(application.Spec.PodGroupPolicy),
				Roles:          roles,
			},
		},
	}

	if getApplicationMode(application) == arksv1.ArksApplicationModeDisaggregated && application.Spec.CoordinationPolicy != nil {
		rbgs.Spec.Template.CoordinationRequirements = buildCoordinationRequirements(application.Spec.CoordinationPolicy)
	}

	return rbgs, nil
}

func (r *ArksApplicationReconciler) buildUnifiedRole(application *arksv1.ArksApplication, model *arksv1.ArksModel) (rbgv1alpha1.RoleSpec, error) {
	if application.Spec.Unified == nil {
		return rbgv1alpha1.RoleSpec{}, fmt.Errorf("unified spec is required for unified mode")
	}
	unified := application.Spec.Unified

	image, err := getApplicationRuntimeImage(application)
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, err
	}

	leaderCommand, err := generateLeaderCommand(application, model)
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, err
	}
	workerCommand, err := generateWorkerCommand(application, model)
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, err
	}

	var replicas int32
	if unified.Replicas != nil {
		replicas = *unified.Replicas
	}
	if replicas < 0 {
		replicas = 0
	}
	lwsSize := int32(unified.Size)
	if lwsSize < 1 {
		lwsSize = 1
	}

	volumes, volumeMounts := buildModelVolumes(model, &unified.InstanceSpec)
	envs := buildRuntimeEnvs(getArksApplicationRuntime(application), unified.InstanceSpec.Env)
	podSpec := buildPodSpec(&unified.InstanceSpec, application.Spec.RuntimeImagePullSecrets, volumes, envs, image, leaderCommand)
	podSpec.Containers = []corev1.Container{
		{
			Name:            "instance",
			Image:           image,
			Command:         leaderCommand,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env:             envs,
			Resources:       unified.InstanceSpec.Resources,
			VolumeMounts:    volumeMounts,
			Ports:           []corev1.ContainerPort{{ContainerPort: 8080}},
			SecurityContext: unified.InstanceSpec.SecurityContext,
			ReadinessProbe:  unified.InstanceSpec.ReadinessProbe,
			LivenessProbe:   unified.InstanceSpec.LivenessProbe,
			StartupProbe:    unified.InstanceSpec.StartupProbe,
		},
	}

	workerPatch, err := json.Marshal(corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:      "instance",
					Command:   workerCommand,
					Resources: corev1.ResourceRequirements{},
				},
			},
		},
	})
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, fmt.Errorf("failed to marshal worker patch: %w", err)
	}

	return rbgv1alpha1.RoleSpec{
		Name:          "unified",
		Replicas:      ptr.To(replicas),
		RestartPolicy: rbgv1alpha1.RecreateRoleInstanceOnPodRestart,
		Workload: rbgv1alpha1.WorkloadSpec{
			APIVersion: "leaderworkerset.x-k8s.io/v1",
			Kind:       "LeaderWorkerSet",
		},
		LeaderWorkerSet: &rbgv1alpha1.LeaderWorkerTemplate{
			Size: &lwsSize,
			PatchWorkerTemplate: &runtime.RawExtension{
				Raw: workerPatch,
			},
		},
		RolloutStrategy: &rbgv1alpha1.RolloutStrategy{
			Type: rbgv1alpha1.RollingUpdateStrategyType,
			RollingUpdate: &rbgv1alpha1.RollingUpdate{
				MaxUnavailable: ptr.To(intstr.FromInt(1)),
				MaxSurge:       ptr.To(intstr.FromInt(0)),
				Partition:      ptr.To(intstr.FromInt(0)),
			},
		},
		TemplateSource: rbgv1alpha1.TemplateSource{
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: unified.InstanceSpec.Annotations,
					Labels:      generateUnifiedLabels(application, arksv1.ArksWorkLoadRoleLeader),
				},
				Spec: podSpec,
			},
		},
	}, nil
}

func (r *ArksApplicationReconciler) buildRouterRole(ctx context.Context, application *arksv1.ArksApplication) (rbgv1alpha1.RoleSpec, error) {
	if application.Spec.Router == nil {
		return rbgv1alpha1.RoleSpec{}, fmt.Errorf("router spec is required to build router role")
	}
	router := application.Spec.Router
	serviceAccountName := router.InstanceSpec.ServiceAccountName
	if serviceAccountName == "" {
		var err error
		serviceAccountName, err = r.applyRouterRBAC(ctx, application)
		if err != nil {
			return rbgv1alpha1.RoleSpec{}, err
		}
	}

	port := router.Port
	if port == 0 {
		port = 8080
	}
	metricPort := router.MetricPort
	if metricPort == 0 {
		metricPort = 9090
	}

	image, err := r.getApplicationRouterImage(application)
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, err
	}
	envs := append([]corev1.EnvVar{}, router.InstanceSpec.Env...)
	var commands []string
	if hasCustomRouter(application) {
		commands = router.CommandOverride
	} else {
		command, err := r.generateRouterCommand(application, port, metricPort)
		if err != nil {
			return rbgv1alpha1.RoleSpec{}, err
		}
		commands = []string{"/bin/bash", "-c", command}
	}

	replicas := int32(1)
	if router.Replicas != nil && *router.Replicas >= 0 {
		replicas = *router.Replicas
	}

	probePort := port
	readinessProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readiness",
				Port: intstr.FromInt(int(probePort)),
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      3,
		FailureThreshold:    120,
	}
	if router.InstanceSpec.ReadinessProbe != nil {
		readinessProbe = router.InstanceSpec.ReadinessProbe
	}

	instance := &router.InstanceSpec
	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: instance.TerminationGracePeriodSeconds,
		ActiveDeadlineSeconds:         instance.ActiveDeadlineSeconds,
		DNSPolicy:                     instance.DNSPolicy,
		DNSConfig:                     instance.DNSConfig,
		AutomountServiceAccountToken:  instance.AutomountServiceAccountToken,
		NodeName:                      instance.NodeName,
		HostNetwork:                   instance.HostNetwork,
		HostPID:                       instance.HostPID,
		HostIPC:                       instance.HostIPC,
		ShareProcessNamespace:         instance.ShareProcessNamespace,
		SecurityContext:               instance.PodSecurityContext,
		Subdomain:                     instance.Subdomain,
		HostAliases:                   instance.HostAliases,
		PriorityClassName:             instance.PriorityClassName,
		Priority:                      instance.Priority,
		RuntimeClassName:              instance.RuntimeClassName,
		EnableServiceLinks:            instance.EnableServiceLinks,
		PreemptionPolicy:              instance.PreemptionPolicy,
		Overhead:                      instance.Overhead,
		TopologySpreadConstraints:     instance.TopologySpreadConstraints,
		SetHostnameAsFQDN:             instance.SetHostnameAsFQDN,
		OS:                            instance.OS,
		HostUsers:                     instance.HostUsers,
		SchedulingGates:               instance.SchedulingGates,
		ResourceClaims:                instance.ResourceClaims,
		ServiceAccountName:            serviceAccountName,
		SchedulerName:                 instance.SchedulerName,
		Affinity:                      instance.Affinity,
		NodeSelector:                  instance.NodeSelector,
		Tolerations:                   instance.Tolerations,
		ImagePullSecrets:              application.Spec.RuntimeImagePullSecrets,
		InitContainers:                instance.InitContainers,
		Volumes:                       instance.Volumes,
		Containers: []corev1.Container{
			{
				Name:            "main",
				Image:           image,
				Command:         commands,
				Resources:       instance.Resources,
				SecurityContext: instance.SecurityContext,
				ReadinessProbe:  readinessProbe,
				LivenessProbe:   instance.LivenessProbe,
				StartupProbe:    instance.StartupProbe,
				Env:             envs,
				Ports:           []corev1.ContainerPort{{ContainerPort: port}},
				VolumeMounts:    instance.VolumeMounts,
			},
		},
	}

	return rbgv1alpha1.RoleSpec{
		Name:          "router",
		Replicas:      ptr.To(replicas),
		RestartPolicy: rbgv1alpha1.RecreateRoleInstanceOnPodRestart,
		Workload: rbgv1alpha1.WorkloadSpec{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		RolloutStrategy: &rbgv1alpha1.RolloutStrategy{
			Type: rbgv1alpha1.RollingUpdateStrategyType,
			RollingUpdate: &rbgv1alpha1.RollingUpdate{
				MaxUnavailable: ptr.To(intstr.FromInt(1)),
				MaxSurge:       ptr.To(intstr.FromInt(0)),
				Partition:      ptr.To(intstr.FromInt(0)),
			},
		},
		TemplateSource: rbgv1alpha1.TemplateSource{
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: instance.Annotations,
					Labels:      generateRouterLabels(application),
				},
				Spec: podSpec,
			},
		},
	}, nil
}

func (r *ArksApplicationReconciler) buildDisaggregatedRole(application *arksv1.ArksApplication, model *arksv1.ArksModel, roleName string) (rbgv1alpha1.RoleSpec, error) {
	image, err := getApplicationRuntimeImage(application)
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, err
	}

	workload := application.Spec.Prefill
	labelsFn := generatePrefillLabels
	if roleName == "decode" {
		workload = application.Spec.Decode
		labelsFn = generateDecodeLabels
	}
	if workload == nil {
		return rbgv1alpha1.RoleSpec{}, fmt.Errorf("%s workload is required", roleName)
	}

	leaderCommand, err := r.generateDisaggregationLeaderCommand(application, model, roleName)
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, err
	}
	workerCommand, err := r.generateDisaggregationWorkerCommand(application, model, roleName)
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, err
	}

	size := workload.Size
	if size < 1 {
		size = 1
	}
	replicas := normalizeReplica(workload.Replicas, 1)

	volumes, volumeMounts := buildModelVolumes(model, &workload.InstanceSpec)
	envs := buildRuntimeEnvs(getArksApplicationRuntime(application), workload.InstanceSpec.Env)

	leaderEnvs := append([]corev1.EnvVar{}, envs...)
	leaderCommands := []string{"/bin/bash", "-c", leaderCommand}
	if len(workload.LeaderCommandOverride) > 0 {
		leaderEnvs = append(leaderEnvs, corev1.EnvVar{Name: "ARKS_LEADER_COMMAND", Value: leaderCommand})
		leaderCommands = workload.LeaderCommandOverride
	}

	workerEnvs := append([]corev1.EnvVar{}, envs...)
	workerCommands := []string{"/bin/bash", "-c", workerCommand}
	if len(workload.WorkerCommandOverride) > 0 {
		workerEnvs = append(workerEnvs, corev1.EnvVar{Name: "ARKS_WORKER_COMMAND", Value: workerCommand})
		workerCommands = workload.WorkerCommandOverride
	}

	podSpec := buildPodSpec(&workload.InstanceSpec, application.Spec.RuntimeImagePullSecrets, volumes, workerEnvs, image, workerCommands)
	podSpec.Containers = []corev1.Container{
		{
			Name:            "main",
			Image:           image,
			Command:         workerCommands,
			Resources:       workload.InstanceSpec.Resources,
			VolumeMounts:    volumeMounts,
			Env:             workerEnvs,
			SecurityContext: workload.InstanceSpec.SecurityContext,
		},
	}

	leaderPatch, err := json.Marshal(corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: workload.InstanceSpec.Annotations,
			Labels:      labelsFn(application, arksv1.ArksWorkLoadRoleLeader),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:           "main",
					Command:        leaderCommands,
					Env:            leaderEnvs,
					ReadinessProbe: workload.InstanceSpec.ReadinessProbe,
					LivenessProbe:  workload.InstanceSpec.LivenessProbe,
					StartupProbe:   workload.InstanceSpec.StartupProbe,
					Ports:          []corev1.ContainerPort{{ContainerPort: 8080}},
				},
			},
		},
	})
	if err != nil {
		return rbgv1alpha1.RoleSpec{}, fmt.Errorf("failed to marshal leader patch: %w", err)
	}

	return rbgv1alpha1.RoleSpec{
		Name:          roleName,
		Replicas:      ptr.To(replicas),
		RestartPolicy: rbgv1alpha1.RecreateRoleInstanceOnPodRestart,
		Workload: rbgv1alpha1.WorkloadSpec{
			APIVersion: "leaderworkerset.x-k8s.io/v1",
			Kind:       "LeaderWorkerSet",
		},
		LeaderWorkerSet: &rbgv1alpha1.LeaderWorkerTemplate{
			Size: ptr.To(int32(size)),
			PatchLeaderTemplate: &runtime.RawExtension{
				Raw: leaderPatch,
			},
		},
		RolloutStrategy: &rbgv1alpha1.RolloutStrategy{
			Type: rbgv1alpha1.RollingUpdateStrategyType,
			RollingUpdate: &rbgv1alpha1.RollingUpdate{
				MaxUnavailable: ptr.To(intstr.FromInt(1)),
				MaxSurge:       ptr.To(intstr.FromInt(0)),
				Partition:      ptr.To(intstr.FromInt(0)),
			},
		},
		TemplateSource: rbgv1alpha1.TemplateSource{
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: workload.InstanceSpec.Annotations,
					Labels:      labelsFn(application, arksv1.ArksWorkLoadRoleWorker),
				},
				Spec: podSpec,
			},
		},
	}, nil
}

func (r *ArksApplicationReconciler) reconcileApplicationService(ctx context.Context, application *arksv1.ArksApplication) (ctrl.Result, error) {
	desiredTarget := r.desiredTrafficTarget(application)

	if application.Status.TrafficTarget == "" {
		if desiredTarget == arksv1.ArksApplicationTrafficTargetEngine {
			application.Status.TrafficTarget = arksv1.ArksApplicationTrafficTargetEngine
		} else {
			application.Status.TrafficTarget = arksv1.ArksApplicationTrafficTargetPending
		}
	}

	modeChanged := application.Status.Mode != "" && application.Status.Mode != getApplicationMode(application)
	if modeChanged {
		application.Status.TrafficTarget = arksv1.ArksApplicationTrafficTargetPending
		updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionFalse, "ModeSwitching", "Mode is being switched, service will be interrupted")
	}

	currentTarget := application.Status.TrafficTarget
	switch {
	case currentTarget == arksv1.ArksApplicationTrafficTargetPending:
		if r.isTrafficTargetReady(application, desiredTarget) {
			if err := r.ensureApplicationService(ctx, application, desiredTarget); err != nil {
				return ctrl.Result{}, err
			}
			application.Status.TrafficTarget = desiredTarget
			application.Status.Mode = getApplicationMode(application)
			updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionTrue, "TargetReady", "Traffic is routed to the ready target")
			return ctrl.Result{}, nil
		}
		updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionFalse, "WaitingForTarget", "Waiting for target component to become ready")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

	case currentTarget == arksv1.ArksApplicationTrafficTargetEngine && desiredTarget == arksv1.ArksApplicationTrafficTargetRouter:
		if r.isTrafficTargetReady(application, arksv1.ArksApplicationTrafficTargetRouter) {
			if err := r.ensureApplicationService(ctx, application, arksv1.ArksApplicationTrafficTargetRouter); err != nil {
				return ctrl.Result{}, err
			}
			application.Status.TrafficTarget = arksv1.ArksApplicationTrafficTargetRouter
			updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionTrue, "RouterReady", "Traffic switched to router")
			return ctrl.Result{}, nil
		}
		updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionFalse, "RouterStarting", "Waiting for router to become ready")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

	case currentTarget == arksv1.ArksApplicationTrafficTargetRouter && desiredTarget == arksv1.ArksApplicationTrafficTargetEngine:
		if !r.isTrafficTargetReady(application, arksv1.ArksApplicationTrafficTargetEngine) {
			updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionFalse, "EngineNotReady", "Waiting for engine to become ready")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		if err := r.ensureApplicationService(ctx, application, arksv1.ArksApplicationTrafficTargetEngine); err != nil {
			return ctrl.Result{}, err
		}
		application.Status.TrafficTarget = arksv1.ArksApplicationTrafficTargetEngine
		updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionTrue, "EngineReady", "Traffic switched to engine")
		return ctrl.Result{Requeue: true}, nil

	default:
		if err := r.ensureApplicationService(ctx, application, currentTarget); err != nil {
			return ctrl.Result{}, err
		}
		if currentTarget == desiredTarget && r.isTrafficTargetReady(application, currentTarget) {
			updateApplicationCondition(application, arksv1.ArksApplicationTrafficTargetReady, corev1.ConditionTrue, "Stable", "Traffic target is stable")
			if !modeChanged {
				application.Status.Mode = getApplicationMode(application)
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ArksApplicationReconciler) desiredTrafficTarget(application *arksv1.ArksApplication) arksv1.ArksApplicationTrafficTarget {
	if getApplicationMode(application) == arksv1.ArksApplicationModeDisaggregated || isApplicationRouterRequested(application) {
		return arksv1.ArksApplicationTrafficTargetRouter
	}
	return arksv1.ArksApplicationTrafficTargetEngine
}

func (r *ArksApplicationReconciler) shouldRenderRouterRole(application *arksv1.ArksApplication) bool {
	if getApplicationMode(application) == arksv1.ArksApplicationModeDisaggregated {
		return true
	}
	// In unified mode the router role is rendered iff the user requested it
	// (spec.router != nil). When the user removes spec.router, the role must
	// be dropped immediately on the next reconcile; the controller still
	// switches Service traffic from router to the engine pods through the
	// TrafficTarget state machine. A brief endpoints gap is possible while
	// the router Deployment terminates in parallel with the Service selector
	// switch, but the engine pods are already running so most traffic
	// continues uninterrupted via the in-cluster proxy's endpoint update.
	return isApplicationRouterRequested(application)
}

func (r *ArksApplicationReconciler) isTrafficTargetReady(application *arksv1.ArksApplication, target arksv1.ArksApplicationTrafficTarget) bool {
	switch target {
	case arksv1.ArksApplicationTrafficTargetEngine:
		return application.Status.Unified.Replicas > 0 &&
			application.Status.Unified.ReadyReplicas == application.Status.Unified.Replicas
	case arksv1.ArksApplicationTrafficTargetRouter:
		if getApplicationMode(application) == arksv1.ArksApplicationModeDisaggregated {
			return application.Status.Router.ReadyReplicas > 0 &&
				application.Status.Prefill.Replicas > 0 &&
				application.Status.Prefill.ReadyReplicas == application.Status.Prefill.Replicas &&
				application.Status.Decode.Replicas > 0 &&
				application.Status.Decode.ReadyReplicas == application.Status.Decode.Replicas
		}
		return application.Status.Router.ReadyReplicas > 0 &&
			application.Status.Unified.Replicas > 0 &&
			application.Status.Unified.ReadyReplicas == application.Status.Unified.Replicas
	default:
		return false
	}
}

func (r *ArksApplicationReconciler) ensureApplicationService(ctx context.Context, application *arksv1.ArksApplication, target arksv1.ArksApplicationTrafficTarget) error {
	serviceName := generateApplicationServiceName(application)
	selector := applicationUnifiedSelector(application)
	port := int32(8080)
	targetPort := intstr.FromInt(8080)
	if target == arksv1.ArksApplicationTrafficTargetRouter {
		selector = applicationRouterSelector(application)
		if application.Spec.Router != nil && application.Spec.Router.Port != 0 {
			targetPort = intstr.FromInt(int(application.Spec.Router.Port))
		}
	}

	service := &corev1.Service{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: application.Namespace}, service); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to query service: %w", err)
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: application.Namespace,
				Name:      serviceName,
				Labels: map[string]string{
					"prometheus-discovery": "true",
					"managed-by":           "arks",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: selector,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       port,
						TargetPort: targetPort,
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
		if err := ctrl.SetControllerReference(application, service, r.Scheme); err != nil {
			return err
		}
		if err := r.Client.Create(ctx, service); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create service: %w", err)
		}
		return nil
	}

	needsUpdate := !apiequality.Semantic.DeepEqual(service.Spec.Selector, selector) ||
		len(service.Spec.Ports) != 1 ||
		service.Spec.Ports[0].Port != port ||
		service.Spec.Ports[0].TargetPort != targetPort
	if !needsUpdate {
		return nil
	}

	patch := client.MergeFrom(service.DeepCopy())
	service.Spec.Selector = selector
	service.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       port,
			TargetPort: targetPort,
			Protocol:   corev1.ProtocolTCP,
		},
	}
	if err := r.Client.Patch(ctx, service, patch); err != nil {
		return fmt.Errorf("failed to patch service: %w", err)
	}
	return nil
}

func (r *ArksApplicationReconciler) syncApplicationStatus(ctx context.Context, application *arksv1.ArksApplication) error {
	application.Status.Unified = arksv1.ArksRoleStatus{}
	application.Status.Router = arksv1.ArksRoleStatus{}
	application.Status.Prefill = arksv1.ArksRoleStatus{}
	application.Status.Decode = arksv1.ArksRoleStatus{}
	application.Status.Replicas = 0
	application.Status.ReadyReplicas = 0
	application.Status.UpdatedReplicas = 0

	rbgs := &rbgv1alpha1.RoleBasedGroupSet{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: application.Name, Namespace: application.Namespace}, rbgs); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to query RBGS status: %w", err)
	}

	rbgList := &rbgv1alpha1.RoleBasedGroupList{}
	if err := r.Client.List(ctx, rbgList, client.InNamespace(application.Namespace)); err != nil {
		return fmt.Errorf("failed to list RBGs: %w", err)
	}

	var rbg *rbgv1alpha1.RoleBasedGroup
	for i := range rbgList.Items {
		if owner := metav1.GetControllerOf(&rbgList.Items[i]); owner != nil && owner.Kind == "RoleBasedGroupSet" && owner.Name == rbgs.Name {
			rbg = &rbgList.Items[i]
			break
		}
	}
	if rbg == nil {
		return nil
	}

	for _, roleStatus := range rbg.Status.RoleStatuses {
		switch roleStatus.Name {
		case "unified":
			application.Status.Unified.Replicas = roleStatus.Replicas
			application.Status.Unified.ReadyReplicas = roleStatus.ReadyReplicas
			if lws, err := r.LWSClient.LeaderworkersetV1().LeaderWorkerSets(application.Namespace).Get(ctx, fmt.Sprintf("%s-unified", rbg.Name), metav1.GetOptions{}); err == nil {
				application.Status.Unified.UpdatedReplicas = lws.Status.UpdatedReplicas
			}
		case "router":
			application.Status.Router.Replicas = roleStatus.Replicas
			application.Status.Router.ReadyReplicas = roleStatus.ReadyReplicas
			if deploy, err := r.KubeClient.AppsV1().Deployments(application.Namespace).Get(ctx, fmt.Sprintf("%s-router", rbg.Name), metav1.GetOptions{}); err == nil {
				application.Status.Router.UpdatedReplicas = deploy.Status.UpdatedReplicas
			}
		case "prefill":
			application.Status.Prefill.Replicas = roleStatus.Replicas
			application.Status.Prefill.ReadyReplicas = roleStatus.ReadyReplicas
			if lws, err := r.LWSClient.LeaderworkersetV1().LeaderWorkerSets(application.Namespace).Get(ctx, fmt.Sprintf("%s-prefill", rbg.Name), metav1.GetOptions{}); err == nil {
				application.Status.Prefill.UpdatedReplicas = lws.Status.UpdatedReplicas
			}
		case "decode":
			application.Status.Decode.Replicas = roleStatus.Replicas
			application.Status.Decode.ReadyReplicas = roleStatus.ReadyReplicas
			if lws, err := r.LWSClient.LeaderworkersetV1().LeaderWorkerSets(application.Namespace).Get(ctx, fmt.Sprintf("%s-decode", rbg.Name), metav1.GetOptions{}); err == nil {
				application.Status.Decode.UpdatedReplicas = lws.Status.UpdatedReplicas
			}
		}
	}

	syncApplicationAggregateStatus(application)
	return nil
}

func syncApplicationAggregateStatus(application *arksv1.ArksApplication) {
	switch getApplicationMode(application) {
	case arksv1.ArksApplicationModeDisaggregated:
		application.Status.Replicas =
			application.Status.Router.Replicas +
				application.Status.Prefill.Replicas +
				application.Status.Decode.Replicas
		application.Status.ReadyReplicas =
			application.Status.Router.ReadyReplicas +
				application.Status.Prefill.ReadyReplicas +
				application.Status.Decode.ReadyReplicas
		application.Status.UpdatedReplicas =
			application.Status.Router.UpdatedReplicas +
				application.Status.Prefill.UpdatedReplicas +
				application.Status.Decode.UpdatedReplicas
	default:
		application.Status.Replicas = application.Status.Unified.Replicas
		application.Status.ReadyReplicas = application.Status.Unified.ReadyReplicas
		application.Status.UpdatedReplicas = application.Status.Unified.UpdatedReplicas
	}
}

func (r *ArksApplicationReconciler) updateApplicationPhase(application *arksv1.ArksApplication) {
	if isArksAppReady(application) {
		application.Status.Phase = string(arksv1.ArksApplicationPhaseRunning)
		application.Status.Mode = getApplicationMode(application)
		updateApplicationCondition(application, arksv1.ArksApplicationReady, corev1.ConditionTrue, "Running", "The LLM service is running")
		return
	}

	if application.Status.Phase != string(arksv1.ArksApplicationPhaseFailed) {
		application.Status.Phase = string(arksv1.ArksApplicationPhaseCreating)
	}
	updateApplicationCondition(application, arksv1.ArksApplicationReady, corev1.ConditionFalse, "WaitingForReady", "Waiting for all active components to become ready")
}

func buildModelVolumes(model *arksv1.ArksModel, instanceSpec *arksv1.ArksInstanceSpec) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: arksApplicationModelVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: model.Spec.Storage.PVC.Name,
				},
			},
		},
	}
	volumes = append(volumes, instanceSpec.Volumes...)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      arksApplicationModelVolumeName,
			MountPath: arksApplicationModelVolumeMountPath,
			ReadOnly:  true,
		},
	}
	volumeMounts = append(volumeMounts, instanceSpec.VolumeMounts...)
	return volumes, volumeMounts
}

func buildRuntimeEnvs(runtime string, envs []corev1.EnvVar) []corev1.EnvVar {
	result := append([]corev1.EnvVar{}, envs...)
	if runtime == string(arksv1.ArksRuntimeSGLang) {
		result = append(result, corev1.EnvVar{
			Name: "LWS_WORKER_INDEX",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.labels['leaderworkerset.sigs.k8s.io/worker-index']",
				},
			},
		})
	}
	return result
}

func buildPodSpec(instanceSpec *arksv1.ArksInstanceSpec, imagePullSecrets []corev1.LocalObjectReference, volumes []corev1.Volume, envs []corev1.EnvVar, image string, command []string) corev1.PodSpec {
	return corev1.PodSpec{
		TerminationGracePeriodSeconds: instanceSpec.TerminationGracePeriodSeconds,
		ActiveDeadlineSeconds:         instanceSpec.ActiveDeadlineSeconds,
		DNSPolicy:                     instanceSpec.DNSPolicy,
		DNSConfig:                     instanceSpec.DNSConfig,
		AutomountServiceAccountToken:  instanceSpec.AutomountServiceAccountToken,
		NodeName:                      instanceSpec.NodeName,
		HostNetwork:                   instanceSpec.HostNetwork,
		HostPID:                       instanceSpec.HostPID,
		HostIPC:                       instanceSpec.HostIPC,
		ShareProcessNamespace:         instanceSpec.ShareProcessNamespace,
		SecurityContext:               instanceSpec.PodSecurityContext,
		Subdomain:                     instanceSpec.Subdomain,
		HostAliases:                   instanceSpec.HostAliases,
		PriorityClassName:             instanceSpec.PriorityClassName,
		Priority:                      instanceSpec.Priority,
		RuntimeClassName:              instanceSpec.RuntimeClassName,
		EnableServiceLinks:            instanceSpec.EnableServiceLinks,
		PreemptionPolicy:              instanceSpec.PreemptionPolicy,
		Overhead:                      instanceSpec.Overhead,
		TopologySpreadConstraints:     instanceSpec.TopologySpreadConstraints,
		SetHostnameAsFQDN:             instanceSpec.SetHostnameAsFQDN,
		OS:                            instanceSpec.OS,
		HostUsers:                     instanceSpec.HostUsers,
		SchedulingGates:               instanceSpec.SchedulingGates,
		ResourceClaims:                instanceSpec.ResourceClaims,
		ServiceAccountName:            instanceSpec.ServiceAccountName,
		SchedulerName:                 instanceSpec.SchedulerName,
		Affinity:                      instanceSpec.Affinity,
		NodeSelector:                  instanceSpec.NodeSelector,
		Tolerations:                   instanceSpec.Tolerations,
		ImagePullSecrets:              imagePullSecrets,
		InitContainers:                instanceSpec.InitContainers,
		Volumes:                       volumes,
		Containers: []corev1.Container{
			{
				Name:            "main",
				Image:           image,
				Command:         command,
				Resources:       instanceSpec.Resources,
				VolumeMounts:    []corev1.VolumeMount{},
				Env:             envs,
				SecurityContext: instanceSpec.SecurityContext,
				ReadinessProbe:  instanceSpec.ReadinessProbe,
				LivenessProbe:   instanceSpec.LivenessProbe,
				StartupProbe:    instanceSpec.StartupProbe,
			},
		},
	}
}

func generateApplicationServiceName(application *arksv1.ArksApplication) string {
	return fmt.Sprintf("arks-application-%s", application.Name)
}

func applicationUnifiedSelector(application *arksv1.ArksApplication) map[string]string {
	return map[string]string{
		arksv1.ArksControllerKeyApplication:  application.Name,
		arksv1.ArksControllerKeyRole:         "unified",
		arksv1.ArksControllerKeyWorkLoadRole: arksv1.ArksWorkLoadRoleLeader,
	}
}

func applicationRouterSelector(application *arksv1.ArksApplication) map[string]string {
	return map[string]string{
		arksv1.ArksControllerKeyApplication: application.Name,
		arksv1.ArksControllerKeyRole:        "router",
	}
}

func generateUnifiedLabels(application *arksv1.ArksApplication, role string) map[string]string {
	labels := map[string]string{}
	if application.Spec.Unified != nil {
		for key, value := range application.Spec.Unified.InstanceSpec.Labels {
			labels[key] = value
		}
	}
	labels[arksv1.ArksControllerKeyApplication] = application.Name
	labels[arksv1.ArksControllerKeyModel] = application.Spec.Model.Name
	labels[arksv1.ArksControllerKeyRole] = "unified"
	labels[arksv1.ArksControllerKeyWorkLoadRole] = role
	return labels
}

func generateRouterLabels(application *arksv1.ArksApplication) map[string]string {
	labels := map[string]string{}
	if application.Spec.Router != nil {
		for key, value := range application.Spec.Router.InstanceSpec.Labels {
			labels[key] = value
		}
	}
	labels[arksv1.ArksControllerKeyApplication] = application.Name
	labels[arksv1.ArksControllerKeyModel] = application.Spec.Model.Name
	labels[arksv1.ArksControllerKeyRole] = "router"
	labels[arksv1.ArksControllerKeySglangRouter] = "true"
	return labels
}

func generatePrefillLabels(application *arksv1.ArksApplication, role string) map[string]string {
	labels := map[string]string{}
	if application.Spec.Prefill != nil {
		for key, value := range application.Spec.Prefill.InstanceSpec.Labels {
			labels[key] = value
		}
	}
	labels[arksv1.ArksControllerKeyApplication] = application.Name
	labels[arksv1.ArksControllerKeyModel] = application.Spec.Model.Name
	labels[arksv1.ArksControllerKeyRole] = "prefill"
	labels[arksv1.ArksControllerKeyWorkLoadRole] = role
	return labels
}

func generateDecodeLabels(application *arksv1.ArksApplication, role string) map[string]string {
	labels := map[string]string{}
	if application.Spec.Decode != nil {
		for key, value := range application.Spec.Decode.InstanceSpec.Labels {
			labels[key] = value
		}
	}
	labels[arksv1.ArksControllerKeyApplication] = application.Name
	labels[arksv1.ArksControllerKeyModel] = application.Spec.Model.Name
	labels[arksv1.ArksControllerKeyRole] = "decode"
	labels[arksv1.ArksControllerKeyWorkLoadRole] = role
	return labels
}

func (r *ArksApplicationReconciler) applyRouterRBAC(ctx context.Context, application *arksv1.ArksApplication) (string, error) {
	rbacName := fmt.Sprintf("%s-sglang-router", application.Name)

	if _, err := r.KubeClient.RbacV1().RoleBindings(application.Namespace).Get(ctx, rbacName, metav1.GetOptions{}); err == nil {
		return rbacName, nil
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: application.Namespace,
			Name:      rbacName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(application, arksv1.GroupVersion.WithKind("ArksApplication")),
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				Resources: []string{"pods"},
				APIGroups: []string{""},
			},
		},
	}
	if _, err := r.KubeClient.RbacV1().Roles(application.Namespace).Create(ctx, role, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("failed to create router role: %w", err)
	}

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: application.Namespace,
			Name:      rbacName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(application, arksv1.GroupVersion.WithKind("ArksApplication")),
			},
		},
	}
	if _, err := r.KubeClient.CoreV1().ServiceAccounts(application.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("failed to create router service account: %w", err)
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: application.Namespace,
			Name:      rbacName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(application, arksv1.GroupVersion.WithKind("ArksApplication")),
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      rbacName,
				Namespace: application.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     rbacName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	if _, err := r.KubeClient.RbacV1().RoleBindings(application.Namespace).Create(ctx, roleBinding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("failed to create router role binding: %w", err)
	}

	return rbacName, nil
}

func (r *ArksApplicationReconciler) getApplicationRouterImage(application *arksv1.ArksApplication) (string, error) {
	if application.Spec.RouterImage != "" {
		return application.Spec.RouterImage, nil
	}

	switch getArksApplicationRuntime(application) {
	case string(arksv1.ArksRuntimeSGLang):
		if image := os.Getenv("ARKS_DEFAULT_SGLANG_ROUTER_IMAGE"); image != "" {
			return image, nil
		}
		if image := os.Getenv("ARKS_DEFAULT_SGLANG_IMAGE"); image != "" {
			return image, nil
		}
		return "lmsysorg/sglang:v0.5.1.post1-cu126", nil
	default:
		return "", errors.New("router is only supported with runtime=sglang")
	}
}

func getApplicationRuntimeImage(application *arksv1.ArksApplication) (string, error) {
	if application.Spec.RuntimeImage != "" {
		return application.Spec.RuntimeImage, nil
	}

	switch getArksApplicationRuntime(application) {
	case string(arksv1.ArksRuntimeVLLM):
		if image := os.Getenv("ARKS_RUNTIME_DEFAULT_VLLM_IMAGE"); image != "" {
			return image, nil
		}
		return "vllm/vllm-openai:v0.8.2", nil
	case string(arksv1.ArksRuntimeSGLang):
		if image := os.Getenv("ARKS_RUNTIME_DEFAULT_SGLANG_IMAGE"); image != "" {
			return image, nil
		}
		return "lmsysorg/sglang:v0.4.5-cu124", nil
	case string(arksv1.ArksRuntimeDynamo):
		if image := os.Getenv("ARKS_RUNTIME_DEFAULT_DYNAMO_IMAGE"); image != "" {
			return image, nil
		}
		return "scitixai/k8s/dynamo:vllm", nil
	default:
		return "", fmt.Errorf("runtime not support")
	}
}

func unifiedRuntimeCommonArgs(application *arksv1.ArksApplication) []string {
	if application.Spec.Unified == nil {
		return nil
	}
	return application.Spec.Unified.RuntimeCommonArgs
}

func generateLeaderCommand(application *arksv1.ArksApplication, model *arksv1.ArksModel) ([]string, error) {
	switch getArksApplicationRuntime(application) {
	case string(arksv1.ArksRuntimeVLLM):
		args := "/bin/bash /vllm-workspace/examples/online_serving/multi-node-serving.sh leader --ray_cluster_size=$(LWS_GROUP_SIZE); python3 -m vllm.entrypoints.openai.api_server --port 8080"
		args = fmt.Sprintf("%s --model %s", args, generateModelPath(model))
		args = fmt.Sprintf("%s --served-model-name %s", args, getServedModelName(application))
		for _, arg := range unifiedRuntimeCommonArgs(application) {
			args = fmt.Sprintf("%s %s", args, arg)
		}
		return []string{"/bin/bash", "-c", args}, nil
	case string(arksv1.ArksRuntimeSGLang):
		args := "python3 -m sglang.launch_server --dist-init-addr $(LWS_LEADER_ADDRESS):20000 --nnodes $(LWS_GROUP_SIZE) --node-rank $(LWS_WORKER_INDEX) --trust-remote-code --host 0.0.0.0 --port 8080"
		args = fmt.Sprintf("%s --model-path %s", args, generateModelPath(model))
		args = fmt.Sprintf("%s --served-model-name %s", args, getServedModelName(application))
		for _, arg := range unifiedRuntimeCommonArgs(application) {
			args = fmt.Sprintf("%s %s", args, arg)
		}
		if !strings.Contains(args, "enable-metrics") {
			args = fmt.Sprintf("%s --enable-metrics", args)
		}
		return []string{"/bin/bash", "-c", args}, nil
	case string(arksv1.ArksRuntimeDynamo):
		args := "dynamo run in=http out=dyn://$(LWS_LEADER_ADDRESS)"
		for _, arg := range unifiedRuntimeCommonArgs(application) {
			args = fmt.Sprintf("%s %s", args, arg)
		}
		return []string{"/bin/bash", "-c", args}, nil
	default:
		return nil, fmt.Errorf("runtime not support")
	}
}

func generateWorkerCommand(application *arksv1.ArksApplication, model *arksv1.ArksModel) ([]string, error) {
	switch getArksApplicationRuntime(application) {
	case string(arksv1.ArksRuntimeVLLM):
		return []string{"/bin/bash", "-c", "/bin/bash /vllm-workspace/examples/online_serving/multi-node-serving.sh worker --ray_address=$(LWS_LEADER_ADDRESS)"}, nil
	case string(arksv1.ArksRuntimeSGLang):
		args := "python3 -m sglang.launch_server --dist-init-addr $(LWS_LEADER_ADDRESS):20000 --nnodes $(LWS_GROUP_SIZE) --node-rank $(LWS_WORKER_INDEX) --trust-remote-code"
		args = fmt.Sprintf("%s --model-path %s", args, generateModelPath(model))
		args = fmt.Sprintf("%s --served-model-name %s", args, getServedModelName(application))
		for _, arg := range unifiedRuntimeCommonArgs(application) {
			args = fmt.Sprintf("%s %s", args, arg)
		}
		if !strings.Contains(args, "enable-metrics") {
			args = fmt.Sprintf("%s --enable-metrics", args)
		}
		return []string{"/bin/bash", "-c", args}, nil
	case string(arksv1.ArksRuntimeDynamo):
		args := fmt.Sprintf("dynamo run in=dyn://$(LWS_LEADER_ADDRESS) out=vllm %s", generateModelPath(model))
		args = fmt.Sprintf("%s --model-name %s", args, getServedModelName(application))
		for _, arg := range unifiedRuntimeCommonArgs(application) {
			args = fmt.Sprintf("%s %s", args, arg)
		}
		return []string{"/bin/bash", "-c", args}, nil
	default:
		return nil, fmt.Errorf("runtime not support")
	}
}

func (r *ArksApplicationReconciler) generateDisaggregationLeaderCommand(application *arksv1.ArksApplication, model *arksv1.ArksModel, roleName string) (string, error) {
	workload := application.Spec.Prefill
	if roleName == "decode" {
		workload = application.Spec.Decode
	}
	if workload == nil {
		return "", fmt.Errorf("%s workload is required", roleName)
	}

	switch getArksApplicationRuntime(application) {
	case string(arksv1.ArksRuntimeSGLang):
		args := "python3 -m sglang.launch_server --dist-init-addr $(LWS_LEADER_ADDRESS):20000 --nnodes $(LWS_GROUP_SIZE) --node-rank $(LWS_WORKER_INDEX) --trust-remote-code --host 0.0.0.0 --port 8080"
		for _, arg := range workload.RuntimeCommonArgs {
			args = fmt.Sprintf("%s %s", args, arg)
		}
		if !strings.Contains(args, "--model-path") {
			args = fmt.Sprintf("%s --model-path %s", args, generateModelPath(model))
		}
		if !strings.Contains(args, "--served-model-name") {
			args = fmt.Sprintf("%s --served-model-name %s", args, getServedModelName(application))
		}
		if !strings.Contains(args, "--disaggregation-mode") {
			args = fmt.Sprintf("%s --disaggregation-mode %s", args, roleName)
		}
		if !strings.Contains(args, "--enable-metrics") {
			args = fmt.Sprintf("%s --enable-metrics", args)
		}
		return args, nil
	default:
		return "", errors.New("unsupported runtime for disaggregated mode")
	}
}

func (r *ArksApplicationReconciler) generateDisaggregationWorkerCommand(application *arksv1.ArksApplication, model *arksv1.ArksModel, roleName string) (string, error) {
	workload := application.Spec.Prefill
	if roleName == "decode" {
		workload = application.Spec.Decode
	}
	if workload == nil {
		return "", fmt.Errorf("%s workload is required", roleName)
	}

	switch getArksApplicationRuntime(application) {
	case string(arksv1.ArksRuntimeSGLang):
		args := "python3 -m sglang.launch_server --dist-init-addr $(LWS_LEADER_ADDRESS):20000 --nnodes $(LWS_GROUP_SIZE) --node-rank $(LWS_WORKER_INDEX) --trust-remote-code"
		args = fmt.Sprintf("%s --model-path %s", args, generateModelPath(model))
		args = fmt.Sprintf("%s --served-model-name %s", args, getServedModelName(application))
		args = fmt.Sprintf("%s --disaggregation-mode %s", args, roleName)
		for _, arg := range workload.RuntimeCommonArgs {
			args = fmt.Sprintf("%s %s", args, arg)
		}
		if !strings.Contains(args, "--enable-metrics") {
			args = fmt.Sprintf("%s --enable-metrics", args)
		}
		return args, nil
	default:
		return "", errors.New("unsupported runtime for disaggregated mode")
	}
}

func (r *ArksApplicationReconciler) generateRouterCommand(application *arksv1.ArksApplication, port, metricPort int32) (string, error) {
	if getArksApplicationRuntime(application) != string(arksv1.ArksRuntimeSGLang) {
		return "", errors.New("router is only supported with runtime=sglang")
	}

	var args string
	if getApplicationMode(application) == arksv1.ArksApplicationModeDisaggregated {
		args = fmt.Sprintf("python3 -m sglang_router.launch_router --pd-disaggregation --service-discovery --service-discovery-port 8080 --host 0.0.0.0 --port %d", port)
		args = fmt.Sprintf("%s --service-discovery-namespace %s", args, application.Namespace)
		args = fmt.Sprintf("%s --prefill-selector", args)
		prefillLabels := generateDisaggregatedSelectorLabels(application, "prefill")
		for _, key := range sortedKeys(prefillLabels) {
			args = fmt.Sprintf("%s %s=%s", args, key, prefillLabels[key])
		}
		args = fmt.Sprintf("%s --decode-selector", args)
		decodeLabels := generateDisaggregatedSelectorLabels(application, "decode")
		for _, key := range sortedKeys(decodeLabels) {
			args = fmt.Sprintf("%s %s=%s", args, key, decodeLabels[key])
		}
	} else {
		args = fmt.Sprintf("python3 -m sglang_router.launch_router --service-discovery --service-discovery-port 8080 --host 0.0.0.0 --port %d", port)
		args = fmt.Sprintf("%s --service-discovery-namespace %s", args, application.Namespace)
		args = fmt.Sprintf("%s --selector", args)
		unifiedLabels := applicationUnifiedSelector(application)
		for _, key := range sortedKeys(unifiedLabels) {
			args = fmt.Sprintf("%s %s=%s", args, key, unifiedLabels[key])
		}
	}

	for _, arg := range application.Spec.Router.RouterArgs {
		args = fmt.Sprintf("%s %s", args, arg)
	}
	if !strings.Contains(args, "-policy") {
		args = fmt.Sprintf("%s --policy cache_aware", args)
	}
	if !strings.Contains(args, "prometheus-port") {
		args = fmt.Sprintf("%s --prometheus-host 0.0.0.0", args)
		args = fmt.Sprintf("%s --prometheus-port %d", args, metricPort)
	}
	return args, nil
}

func getServedModelName(application *arksv1.ArksApplication) string {
	if application.Spec.ServedModelName != "" {
		return application.Spec.ServedModelName
	}
	return application.Spec.Model.Name
}

func (r *ArksApplicationReconciler) patchApplicationStatus(ctx context.Context, original, updated *arksv1.ArksApplication) error {
	if apiequality.Semantic.DeepEqual(original.Status, updated.Status) {
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &arksv1.ArksApplication{}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(updated), current); err != nil {
			return client.IgnoreNotFound(err)
		}
		current.Status = updated.Status
		return r.Client.Status().Update(ctx, current)
	})
}

func (r *ArksApplicationReconciler) removeFinalizerWithRetry(ctx context.Context, application *arksv1.ArksApplication) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &arksv1.ArksApplication{}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(application), current); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !hasFinalizer(current, arksApplicationControllerFinalizer) {
			return nil
		}
		removeFinalizer(current, arksApplicationControllerFinalizer)
		return r.Client.Update(ctx, current)
	})
}

func getApplicationMode(application *arksv1.ArksApplication) arksv1.ArksApplicationMode {
	if application.Spec.Mode == "" {
		return arksv1.ArksApplicationModeUnified
	}
	return application.Spec.Mode
}

func isApplicationRouterRequested(application *arksv1.ArksApplication) bool {
	if getApplicationMode(application) == arksv1.ArksApplicationModeDisaggregated {
		return true
	}
	return application.Spec.Router != nil
}

func getArksApplicationRuntime(application *arksv1.ArksApplication) string {
	if application.Spec.Runtime == "" {
		return string(arksv1.ArksRuntimeDefault)
	}
	return application.Spec.Runtime
}

func (r *ArksApplicationReconciler) requestsForModel(ctx context.Context, obj client.Object) []ctrl.Request {
	model, ok := obj.(*arksv1.ArksModel)
	if !ok {
		return nil
	}

	var apps arksv1.ArksApplicationList
	if err := r.Client.List(ctx, &apps,
		client.InNamespace(model.Namespace),
		client.MatchingFields{arksApplicationModelField: model.Name},
	); err != nil {
		klog.Errorf("failed to list applications referencing model %s/%s: %v", model.Namespace, model.Name, err)
		return nil
	}

	requests := make([]ctrl.Request, 0, len(apps.Items))
	for i := range apps.Items {
		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      apps.Items[i].Name,
				Namespace: apps.Items[i].Namespace,
			},
		})
	}
	return requests
}

func (r *ArksApplicationReconciler) requestsForRBG(ctx context.Context, obj client.Object) []ctrl.Request {
	rbg, ok := obj.(*rbgv1alpha1.RoleBasedGroup)
	if !ok {
		return nil
	}

	rbgsRef := metav1.GetControllerOf(rbg)
	if rbgsRef == nil || rbgsRef.Kind != "RoleBasedGroupSet" {
		return nil
	}

	rbgs := &rbgv1alpha1.RoleBasedGroupSet{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: rbgsRef.Name, Namespace: rbg.Namespace}, rbgs); err != nil {
		return nil
	}

	appRef := metav1.GetControllerOf(rbgs)
	if appRef == nil || appRef.Kind != "ArksApplication" {
		return nil
	}

	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Name:      appRef.Name,
			Namespace: rbg.Namespace,
		},
	}}
}

func checkApplicationCondition(application *arksv1.ArksApplication, conditionType arksv1.ArksApplicationConditionType) bool {
	for i := range application.Status.Conditions {
		if application.Status.Conditions[i].Type == conditionType {
			return application.Status.Conditions[i].Status == corev1.ConditionTrue
		}
	}
	return false
}

func updateApplicationCondition(application *arksv1.ArksApplication, conditionType arksv1.ArksApplicationConditionType, conditionStatus corev1.ConditionStatus, reason, message string) {
	for i := range application.Status.Conditions {
		if application.Status.Conditions[i].Type == conditionType {
			application.Status.Conditions[i].Status = conditionStatus
			application.Status.Conditions[i].Reason = reason
			application.Status.Conditions[i].Message = message
			application.Status.Conditions[i].LastTransitionTime = metav1.Now()
			return
		}
	}

	application.Status.Conditions = append(application.Status.Conditions, arksv1.ArksApplicationCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

func initializeApplicationCondition(application *arksv1.ArksApplication) {
	if application.Status.Conditions != nil {
		return
	}
	application.Status.Conditions = []arksv1.ArksApplicationCondition{
		{
			Type:               arksv1.ArksApplicationPrecheck,
			Status:             corev1.ConditionFalse,
			Reason:             "NewIncomming",
			Message:            "Wait the controller to check the application",
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               arksv1.ArksApplicationLoaded,
			Status:             corev1.ConditionFalse,
			Reason:             "NewIncomming",
			Message:            "Wait the controller to load the model",
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               arksv1.ArksApplicationReady,
			Status:             corev1.ConditionFalse,
			Reason:             "NewIncomming",
			Message:            "Wait the controller to check the application status",
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               arksv1.ArksApplicationTrafficTargetReady,
			Status:             corev1.ConditionFalse,
			Reason:             "NewIncomming",
			Message:            "Wait the controller to route traffic to a ready target",
			LastTransitionTime: metav1.Now(),
		},
	}
}

func generateDisaggregatedSelectorLabels(application *arksv1.ArksApplication, roleName string) map[string]string {
	return map[string]string{
		arksv1.ArksControllerKeyApplication:  application.Name,
		arksv1.ArksControllerKeyModel:        application.Spec.Model.Name,
		arksv1.ArksControllerKeyRole:         roleName,
		arksv1.ArksControllerKeyWorkLoadRole: arksv1.ArksWorkLoadRoleLeader,
	}
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
