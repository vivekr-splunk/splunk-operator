package genai

import (
	"context"
	"fmt"
	"os"
	"reflect"

	enterpriseApi "github.com/vivekrsplunk/splunk-operator/api/v4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SaisServiceReconciler is an interface for reconciling the SaisService and its associated resources.
type SaisServiceReconciler interface {
	Reconcile(ctx context.Context) error
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
	eventRecorder   record.EventRecorder
}

// NewSaisServiceReconciler creates a new instance of SaisServiceReconciler.
func NewSaisServiceReconciler(c client.Client, genAIDeployment *enterpriseApi.GenAIDeployment, eventRecorder record.EventRecorder) SaisServiceReconciler {
	return &saisServiceReconcilerImpl{
		Client:          c,
		genAIDeployment: genAIDeployment,
		eventRecorder:   eventRecorder,
	}
}

// Reconcile manages the complete reconciliation logic for the SaisService and returns its status.
func (r *saisServiceReconcilerImpl) Reconcile(ctx context.Context) error {
	status := enterpriseApi.SaisServiceStatus{}

	// Reconcile the ServiceAccount for SaisService if specified
	if err := r.ReconcileServiceAccount(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile ServiceAccount"
		return err
	}

	// Reconcile the Secret for SaisService
	if err := r.ReconcileSecret(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Secret"
		return err
	}

	// Reconcile the ConfigMap for SaisService
	if err := r.ReconcileConfigMap(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile ConfigMap"
		return err
	}

	// Reconcile the Deployment for SaisService
	if err := r.ReconcileDeployment(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Deployment"
		return err
	}

	// Reconcile the Service for SaisService
	if err := r.ReconcileService(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Service"
		return err
	}

	// Write to event recorder
	r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "Reconciliation", "SaisService reconciliation completed successfully")

	// If all reconciliations succeed, update the status to Running
	status.Status = "Running"
	status.Message = "SaisService is running successfully"
	return nil
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
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "ReconciliationError", fmt.Sprintf("Failed to create ServiceAccount: %v", err))
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
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "ReconciliationError", fmt.Sprintf("Failed to create Secret: %v", err))
			return fmt.Errorf("failed to create Secret: %w", err)
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
			"IAC_URL":             "auth.playground.scs.splunk.com",
			"API_GATEWAY_URL":     "api.playground.scs.splunk.com",
			"PLATFORM_URL":        "ml-platform-cyclops.dev.svc.splunk8s.io",
			"TELEMETRY_URL":       "https://telemetry-splkmobile.kube-bridger",
			"TELEMETRY_ENV":       "",
			"TELEMETRY_REGION":    "region-iad10",
			"ENABLE_AUTHZ":        "false",
			"AUTH_PROVIDER":       "",
			"SCS_TOKEN" :          "your-scs-token",
			"SCPAUTH_SECRET_PATH": "/etc/sais-service-secret",
		},
	}

	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, configMap); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "ReconciliationError", fmt.Sprintf("Failed to create ConfigMap: %v", err))
			return fmt.Errorf("failed to create ConfigMap: %w", err)
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
		"component": r.genAIDeployment.Name,
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
	// Read the AWS_ROLE_ARN from the environment
	awsRoleArn := os.Getenv("AWS_ROLE_ARN")

	s3Bucket := r.genAIDeployment.Spec.Bucket.Path

	annotations := map[string]string{
		"splunk8s.io.vault/init-container": "false",
		"prometheus.io/port":               "8080",
		"prometheus.io/path":               "/metrics",
		"prometheus.io/scheme":             "http",
	}
	
	if r.genAIDeployment.Spec.ServiceAccount != "" {
		serviceAccount := &corev1.ServiceAccount{}
		namespacedName := types.NamespacedName{
			Name: r.genAIDeployment.Spec.ServiceAccount,
			Namespace : r.genAIDeployment.GetNamespace(),
		}
		err := r.Client.Get(ctx, namespacedName, serviceAccount)
		if err != nil {
			return err
		}
		value, exists := serviceAccount.Annotations["iam.amazonaws.com/role"] 
		if exists {
			annotations["iam.amazonaws.com/role"] = value 
		}
	}

	// Set the "iam.amazonaws.com/role" only if AWS_ROLE_ARN is not empty
	if awsRoleArn != "" {
		annotations["iam.amazonaws.com/role"] = awsRoleArn
	}

	// Define the desired environment variables
	configMapEnvVars := []corev1.EnvVar{
		{Name: "IAC_URL", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "IAC_URL",
			}}},
		{Name: "API_GATEWAY_URL", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "API_GATEWAY_URL",
			}}},
		{Name: "PLATFORM_URL", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "PLATFORM_URL",
			}}},
		{Name: "TELEMETRY_URL", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "TELEMETRY_URL",
			}}},
		{Name: "TELEMETRY_ENV", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "TELEMETRY_ENV",
			}}},
		{Name: "TELEMETRY_REGION", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "TELEMETRY_REGION",
			}}},
		{Name: "ENABLE_AUTHZ", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "ENABLE_AUTHZ",
			}}},
		{Name: "AUTH_PROVIDER", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "AUTH_PROVIDER",
			}}},
		{Name: "SCPAUTH_SECRET_PATH", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "SCPAUTH_SECRET_PATH",
			}}},
		{Name: "SCS_TOKEN", ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: r.genAIDeployment.Spec.SaisService.ConfigMapRef,
				Key:                  "SCS_TOKEN",
			}}},
		{Name: "S3_BUCKET", Value: s3Bucket}, //FIXME
	}

	// Add the secret as a volume
	secretName := "sais-service-secret"
	secretMountPath := "/etc/sais-service-secret" // Adjust the mount path as needed

	// Define the volume for the secret
	secretVolume := corev1.Volume{
		Name: secretName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
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
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					// Add the ServiceAccount only if it is specified in the spec
					ServiceAccountName: getServiceAccountName(r.genAIDeployment.Spec.ServiceAccount),
					Containers: []corev1.Container{
						{
							Name:      "sais-container",
							Image:     r.genAIDeployment.Spec.SaisService.Image,
							Resources: r.genAIDeployment.Spec.SaisService.Resources,
							Env:       configMapEnvVars, // Use the desired environment variables
							Command:   []string{"python"},
							Args:      []string{"-m", "uvicorn", "--host", "0.0.0.0", "server.main:app", "--port", "8080"},
							Ports: []corev1.ContainerPort{ // Add the port mapping
								{
									ContainerPort: 8080,
								},
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

	// Check if the volume already exists; if not, add it
	volumeExists := false
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == secretName {
			volumeExists = true
			break
		}
	}
	if !volumeExists {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, secretVolume)
	}

	// Define the volume mount for the secret
	secretVolumeMount := corev1.VolumeMount{
		Name:      secretName,
		MountPath: secretMountPath,
		ReadOnly:  true,
	}

	// Attach the volume mount to the first container in the deployment
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		container := &deployment.Spec.Template.Spec.Containers[0]
		volumeMountExists := false
		for _, mount := range container.VolumeMounts {
			if mount.Name == secretName {
				volumeMountExists = true
				break
			}
		}
		if !volumeMountExists {
			container.VolumeMounts = append(container.VolumeMounts, secretVolumeMount)
		}

		// Define the environment variable for the secret path
		envVarExists := false
		for _, envVar := range container.Env {
			if envVar.Name == "SCPAUTH_SECRET_PATH" {
				envVarExists = true
				break
			}
		}
		if !envVarExists {
			secretEnvVar := corev1.EnvVar{
				Name:  "SCPAUTH_SECRET_PATH",
				Value: secretMountPath,
			}
			container.Env = append(container.Env, secretEnvVar)
		}
	}

	err := r.AddEnvVarsFromSecretToDeployment(ctx, deployment, secretName, "sais-container")
	if err != nil {
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "ReconciliationError", fmt.Sprintf("Failed to add environment variables from secret: %v", err))
		return fmt.Errorf("failed to add environment variables from secret: %w", err)
	}

	// Fetch the existing Deployment
	existingDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Name: deployment.Name, Namespace: deployment.Namespace}, existingDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		// Deployment does not exist, so create it
		if err := r.Create(ctx, deployment); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "CreateDeploymentFailed", fmt.Sprintf("Failed to create Deployment: %v", err))
			return fmt.Errorf("failed to create Deployment: %w", err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "CreatedDeployment", fmt.Sprintf("Successfully created Deployment %s", deployment.Name))
		return nil
	}

	// Only update the allowed fields within spec.template if needed
	updated := false

	// Update the ServiceAccountName only if it is provided in the spec
	if getServiceAccountName(r.genAIDeployment.Spec.ServiceAccount) != "" &&
		existingDeployment.Spec.Template.Spec.ServiceAccountName != r.genAIDeployment.Spec.ServiceAccount {
		existingDeployment.Spec.Template.Spec.ServiceAccountName = r.genAIDeployment.Spec.ServiceAccount
		updated = true
	}
	// Check for changes in environment variables or secret volume
	if !envVarsEqual(existingDeployment.Spec.Template.Spec.Containers[0].Env, configMapEnvVars) {
		// Update environment variables
		existingDeployment.Spec.Template.Spec.Containers[0].Env = configMapEnvVars
		updated = true
	}

	// Add logic to update other fields if necessary...

	if updated {
		// Update the Deployment with the changes
		if err := r.Update(ctx, existingDeployment); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "UpdateDeploymentFailed", fmt.Sprintf("Failed to update Deployment: %v", err))
			return fmt.Errorf("failed to update Deployment: %w", err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdatedDeployment", fmt.Sprintf("Successfully updated Deployment %s", existingDeployment.Name))
	} else {
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "ReconcileDeployment", "No changes detected, Deployment is up to date")
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
			Annotations: map[string]string{
				"gateway-v1-openapi3": "enabled",
				"gateway.splunk8s.io/external-name": "saia-api",
				"gateway.splunk8s.io/recommendedVersion": "v1alpha1",
				"gateway.splunk8s.io/service-name": "saia-api",
				"gateway.splunk8s.io/versions": "v1alpha1",
				"prometheus.io/port": "8080",
				"prometheus.io/path": "/metrics",
				"prometheus.io/scheme": "http",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":        "saia-api",
				"deployment": r.genAIDeployment.Name,
				"component": r.genAIDeployment.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name: "http-saia-api",
					TargetPort: intstr.FromInt(8080),
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name: "http-saia-api-gateway8443",
					TargetPort: intstr.FromInt(8080),
					Port:     8443,
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

// AddEnvVarsFromSecretToDeployment adds environment variables from a secret to a deployment's container
func (r *saisServiceReconcilerImpl) AddEnvVarsFromSecretToDeployment(ctx context.Context, deployment *appsv1.Deployment, secretName, containerName string) error {

	namespacedName := types.NamespacedName{
		Namespace: r.genAIDeployment.Namespace,
		Name:      secretName,
	}
	secret := &corev1.Secret{}
	// Fetch the secret
	err := r.Get(ctx, namespacedName, secret)
	if err != nil {
		return fmt.Errorf("failed to get secret: %v", err)
	}

	// Find the specified container
	var container *corev1.Container
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == containerName {
			container = &deployment.Spec.Template.Spec.Containers[i]
			break
		}
	}
	if container == nil {
		return fmt.Errorf("container %s not found", containerName)
	}

	// Add environment variables from the secret
	envVars := []corev1.EnvVar{}
	for key, value := range secret.Data {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: string(value),
		})
	}
	container.Env = append(container.Env, envVars...)
	return nil
}

// Helper function to get the ServiceAccountName if provided, or return an empty string
func getServiceAccountName(serviceAccount string) string {
	if serviceAccount != "" {
		return serviceAccount
	}
	return ""
}

// Helper function to compare environment variables
func envVarsEqual(existingEnvVars, desiredEnvVars []corev1.EnvVar) bool {
	if len(existingEnvVars) != len(desiredEnvVars) {
		return false
	}

	existingMap := make(map[string]string)
	for _, env := range existingEnvVars {
		existingMap[env.Name] = env.Value
	}

	for _, env := range desiredEnvVars {
		if existingMap[env.Name] != env.Value {
			return false
		}
	}

	return true
}
