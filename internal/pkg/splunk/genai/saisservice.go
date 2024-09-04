package genai

import (
	"context"
	"fmt"
	"reflect"

	enterpriseApi "github.com/vivekrsplunk/splunk-operator/api/v4"
	splutil "github.com/vivekrsplunk/splunk-operator/internal/pkg/splunk/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SaisServiceReconciler is an interface for reconciling the SaisService and its associated resources.
type SaisServiceReconciler interface {
	Reconcile(ctx context.Context) (enterpriseApi.SaisServiceStatus, error)
	ReconcileServiceAccount(ctx context.Context) error
	ReconcileSecret(ctx context.Context) error
	ReconcileConfigMap(ctx context.Context) error
	ReconcileDeployment(ctx context.Context) error
	ReconcileService(ctx context.Context) error
}

// saisServiceReconcilerImpl is the concrete implementation of SaisServiceReconciler.
type saisServiceReconcilerImpl struct {
	client.Client
	genAIDeployment *enterpriseApi.GenAIDeployment
	eventRecorder   *splutil.K8EventPublisher
}

// NewSaisServiceReconciler creates a new instance of SaisServiceReconciler.
func NewSaisServiceReconciler(c client.Client, genAIDeployment *enterpriseApi.GenAIDeployment, eventRecorder *splutil.K8EventPublisher) SaisServiceReconciler {
	return &saisServiceReconcilerImpl{
		Client:          c,
		genAIDeployment: genAIDeployment,
		eventRecorder:   eventRecorder,
	}
}

// Reconcile manages the complete reconciliation logic for the SaisService and returns its status.
func (r *saisServiceReconcilerImpl) Reconcile(ctx context.Context) (enterpriseApi.SaisServiceStatus, error) {
	status := enterpriseApi.SaisServiceStatus{}

	// Reconcile the ServiceAccount for SaisService if specified
	if err := r.ReconcileServiceAccount(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile ServiceAccount"
		return status, err
	}

	// Reconcile the Secret for SaisService
	if err := r.ReconcileSecret(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Secret"
		return status, err
	}

	// Reconcile the ConfigMap for SaisService
	if err := r.ReconcileConfigMap(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile ConfigMap"
		return status, err
	}

	// Reconcile the Deployment for SaisService
	if err := r.ReconcileDeployment(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Deployment"
		return status, err
	}

	// Reconcile the Service for SaisService
	if err := r.ReconcileService(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Service"
		return status, err
	}

	// Write to event recorder
	r.eventRecorder.Warning(ctx, "Reconciliation", "SaisService reconciliation completed successfully")

	// If all reconciliations succeed, update the status to Running
	status.Status = "Running"
	status.Message = "SaisService is running successfully"
	return status, nil
}

func (r *saisServiceReconcilerImpl) ReconcileServiceAccount(ctx context.Context) error {
	if r.genAIDeployment.Spec.ServiceAccount == "" {
		return nil
	}

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.genAIDeployment.Spec.ServiceAccount,
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
	}

	existingSA := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Name: serviceAccount.Name, Namespace: serviceAccount.Namespace}, existingSA)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, serviceAccount); err != nil {
			r.eventRecorder.Warning(ctx, "ReconciliationError", fmt.Sprintf("Failed to create ServiceAccount: %v", err))
			return fmt.Errorf("failed to create ServiceAccount: %w", err)
		}
	}
	return nil
}

func (r *saisServiceReconcilerImpl) ReconcileSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sais-service-secret",
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Data: map[string][]byte{
			"authKey": []byte("your-secret-auth-key"),
		},
	}

	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, existingSecret)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, secret); err != nil {
			r.eventRecorder.Warning(ctx, "ReconciliationError", fmt.Sprintf("Failed to create Secret: %v", err))
			return fmt.Errorf("failed to create Secret: %w", err)
		}
	} else if !reflect.DeepEqual(secret.Data, existingSecret.Data) {
		existingSecret.Data = secret.Data
		if err := r.Update(ctx, existingSecret); err != nil {
			r.eventRecorder.Warning(ctx, "ReconciliationError", fmt.Sprintf("Failed to update Secret: %v", err))
			return fmt.Errorf("failed to update Secret: %w", err)
		}
	}
	return nil
}

func (r *saisServiceReconcilerImpl) ReconcileConfigMap(ctx context.Context) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sais-service-config",
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Data: map[string]string{
			"configKey": "configValue",
		},
	}

	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, configMap); err != nil {
			r.eventRecorder.Warning(ctx, "ReconciliationError", fmt.Sprintf("Failed to create ConfigMap: %v", err))
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
	} else if !reflect.DeepEqual(configMap.Data, existingConfigMap.Data) {
		existingConfigMap.Data = configMap.Data
		if err := r.Update(ctx, existingConfigMap); err != nil {
			r.eventRecorder.Warning(ctx, "ReconciliationError", fmt.Sprintf("Failed to update ConfigMap: %v", err))
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}
	}
	return nil
}

// ReconcileDeployment reconciles the deployment for the SAIS service.
// It creates or updates the deployment based on the provided specifications.
// The deployment is created with the specified replicas and labels.
// If GPU support is required, it adds node selector and tolerations for GPU nodes.
// The deployment is associated with the owner reference of the GenAIDeployment.
// The pod template of the deployment includes the specified annotations and labels.
// The container within the pod template is configured with the specified image and resources.
// It also sets environment variables for various URLs and authentication settings.
// The deployment is created or updated based on the existing deployment's specifications.
// Returns an error if there is a failure in creating or updating the deployment.
func (r *saisServiceReconcilerImpl) ReconcileDeployment(ctx context.Context) error {
	labels := map[string]string{
		"app":        "saia-api",
		"area":       "ml",
		"team":       "ml",
		"version":    "v1alpha1",
		"deployment": r.genAIDeployment.Name,
	}

	// Define node selector and tolerations for GPU support
	nodeSelector := map[string]string{}
	tolerations := []corev1.Toleration{}

	if r.genAIDeployment.Spec.RequireGPU {
		// Assuming nodes with GPU support are labeled as "kubernetes.io/gpu: true"
		nodeSelector["kubernetes.io/gpu"] = "true"

		// Add a toleration for GPU nodes if needed
		tolerations = append(tolerations, corev1.Toleration{
			Key:      "nvidia.com/gpu",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
	annotaions := map[string]string{
		"splunk8s.io.vault/init-container": "true",
		"prometheus.io/port":               "8080",
		"prometheus.io/path":               "/metrics",
		"prometheus.io/scheme":             "http",
		"iam.amazonaws.com/role":           "arn:aws:iam::1.2234567e+07:role/ai-assistant-role-0",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sais-service", r.genAIDeployment.Name),
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r.genAIDeployment.Spec.SaisService.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotaions,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: r.genAIDeployment.Spec.ServiceAccount,
					Containers: []corev1.Container{
						{
							Name:      "sais-container",
							Image:     r.genAIDeployment.Spec.SaisService.Image,
							Resources: r.genAIDeployment.Spec.SaisService.Resources,
							Env: []corev1.EnvVar{
								{Name: "IAC_URL", Value: "auth.playground.scs.splunk.com"},
								{Name: "API_GATEWAY_URL", Value: "api.playground.scs.splunk.com"},
								{Name: "PLATFORM_URL", Value: "ml-platform-cyclops.dev.svc.splunk8s.io"},
								{Name: "TELEMETRY_URL", Value: "https://telemetry-splkmobile.kube-bridger"},
								{Name: "TELEMETRY_ENV", Value: "local"},
								{Name: "TELEMETRY_REGION", Value: "region-iad10"},
								{Name: "ENABLE_AUTHZ", Value: "false"},
								{Name: "AUTH_PROVIDER", Value: "scp"},
							},
							/*VolumeMounts: []corev1.VolumeMount{
								{
									Name:      r.genAIDeployment.Spec.SaisService.Volume.Name,
									MountPath: "/data",
								},
							}, */
						},
					},
					Affinity:                  &r.genAIDeployment.Spec.SaisService.Affinity,
					Tolerations:               tolerations,
					TopologySpreadConstraints: r.genAIDeployment.Spec.SaisService.TopologySpreadConstraints,
				},
			},
		},
	}

	existingDeployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: deployment.Name, Namespace: deployment.Namespace}, existingDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, deployment); err != nil {
			return fmt.Errorf("failed to create Deployment: %w", err)
		}
	} else if !reflect.DeepEqual(deployment.Spec, existingDeployment.Spec) {
		existingDeployment.Spec = deployment.Spec
		if err := r.Update(ctx, existingDeployment); err != nil {
			return fmt.Errorf("failed to update Deployment: %w", err)
		}
	}
	return nil
}

// ReconcileService reconciles the service for the GenAIDeployment.
// It creates a new service if it doesn't exist, or updates the existing service if the spec has changed.
// The service is created with the name "<GenAIDeploymentName>-sais-service" in the same namespace as the GenAIDeployment.
// The service is associated with the GenAIDeployment as an owner reference.
// The service selector is set to match the labels "app=sais-service" and "deployment=<GenAIDeploymentName>".
// The service exposes port 80 with TCP protocol.
// If an error occurs during creation or update of the service, it is returned.
// If the service already exists and the spec hasn't changed, no action is taken.
func (r *saisServiceReconcilerImpl) ReconcileService(ctx context.Context) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sais-service", r.genAIDeployment.Name),
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":        "sais-service",
				"deployment": r.genAIDeployment.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	existingService := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, existingService)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, service); err != nil {
			return fmt.Errorf("failed to create Service: %w", err)
		}
	} else if !reflect.DeepEqual(service.Spec, existingService.Spec) {
		existingService.Spec = service.Spec
		if err := r.Update(ctx, existingService); err != nil {
			return fmt.Errorf("failed to update Service: %w", err)
		}
	}
	return nil
}