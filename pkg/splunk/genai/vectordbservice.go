package genai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"

	enterpriseApi "github.com/vivekrsplunk/splunk-operator/api/v4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VectorDbReconciler is an interface for reconciling the VectorDbService and its associated resources.
type VectorDbReconciler interface {
	Reconcile(ctx context.Context) error
	ReconcileSecret(ctx context.Context) error
	ReconcileConfigMap(ctx context.Context) error
	ReconcileStatefulSet(ctx context.Context) error
	ReconcileServices(ctx context.Context) error
}

// vectorDbReconcilerImpl is the concrete implementation of VectorDbReconciler.
type vectorDbReconcilerImpl struct {
	client.Client
	genAIDeployment *enterpriseApi.GenAIDeployment
	eventRecorder   record.EventRecorder
}

// NewVectorDbReconciler creates a new instance of VectorDbReconciler.
func NewVectorDbReconciler(c client.Client, genAIDeployment *enterpriseApi.GenAIDeployment, eventRecorder record.EventRecorder) VectorDbReconciler {
	return &vectorDbReconcilerImpl{
		Client:          c,
		genAIDeployment: genAIDeployment,
		eventRecorder:   eventRecorder,
	}
}

// Reconcile manages the complete reconciliation logic for the VectorDbService and returns its status.
func (r *vectorDbReconcilerImpl) Reconcile(ctx context.Context) error {
	status := enterpriseApi.VectorDbStatus{}
	if !r.genAIDeployment.Spec.VectorDbService.Enabled {
		status.Status = "Success"
		status.Message = "VectorDbService is not enabled"
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "Reconcile", "VectorDbService is not enabled")
		return nil
	}

	// Reconcile the Secret for Weaviate
	if err := r.ReconcileSecret(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Secret"
		return err
	}

	// Reconcile the ConfigMap for Weaviate
	if err := r.ReconcileConfigMap(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile ConfigMap"
		return err
	}

	// Reconcile PVC for Weaviate StatefulSet
	if err := r.ReconcilePVC(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile PVCs"
		return fmt.Errorf("failed to reconcile PVC: %w", err)
	}

	// Reconcile the StatefulSet for Weaviate
	if err := r.ReconcileStatefulSet(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile StatefulSet"
		return err
	}

	// Reconcile the Services for Weaviate
	if err := r.ReconcileServices(ctx); err != nil {
		status.Status = "Error"
		status.Message = "Failed to reconcile Services"
		return err
	}

	// If all reconciliations succeed, update the status to Running
	status.Status = "Running"
	status.Message = "Weaviate is running successfully"
	return nil
}

func (r *vectorDbReconcilerImpl) ReconcileSecret(ctx context.Context) error {
	// Get the SecretRef from the VectorDbService spec
	secretName := r.genAIDeployment.Spec.VectorDbService.SecretRef
	if secretName.Name == "" {
		err := fmt.Errorf("SecretRef is not specified in the VectorDbService spec")
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "MissingSecretRef", err.Error())
		return err
	}

	// Fetch the existing Secret using the SecretRef
	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secretName.Name, Namespace: r.genAIDeployment.Namespace}, existingSecret)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "GetSecretFailed", fmt.Sprintf("Failed to get existing Secret %s: %v", secretName, err))
			return fmt.Errorf("failed to get existing Secret: %w", err)
		}
		// Secret does not exist, return an error indicating it is required
		err := fmt.Errorf("secret '%s' referenced by SecretRef does not exist", secretName)
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "SecretNotFound", err.Error())
		return err
	}

	// Ensure that the Secret has the expected data keys
	requiredKeys := []string{"AUTHENTICATION_APIKEY_ENABLED", "AUTHENTICATION_APIKEY_ALLOWED_KEYS"}
	for _, key := range requiredKeys {
		if _, exists := existingSecret.Data[key]; !exists {
			err := fmt.Errorf("secret '%s' is missing required key: %s", secretName, key)
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "SecretInvalid", err.Error())
			return err
		}
	}

	// Add OwnerReference to the Secret if not already set
	ownerRef := metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment"))
	if !isOwnerReferenceSet(existingSecret.OwnerReferences, ownerRef) {
		existingSecret.OwnerReferences = append(existingSecret.OwnerReferences, *ownerRef)
		if err := r.Update(ctx, existingSecret); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "UpdateSecretFailed", fmt.Sprintf("Failed to update Secret with OwnerReference: %v", err))
			return fmt.Errorf("failed to update Secret with OwnerReference: %w", err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdatedSecret", fmt.Sprintf("Successfully updated Secret %s with OwnerReference", secretName))
	}

	return nil
}

// isOwnerReferenceSet checks if the owner reference is already set
func isOwnerReferenceSet(ownerRefs []metav1.OwnerReference, ownerRef *metav1.OwnerReference) bool {
	for _, ref := range ownerRefs {
		if ref.UID == ownerRef.UID {
			return true
		}
	}
	return false
}

func (r *vectorDbReconcilerImpl) ReconcileConfigMap(ctx context.Context) error {
	desiredConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "weaviate-config",
			Namespace: r.genAIDeployment.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "weaviate",
				"app.kubernetes.io/managed-by": "sok",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Data: map[string]string{
			"conf.yaml": `
authentication:
  anonymous_access:
    enabled: true
  oidc:
    enabled: false
authorization:
  admin_list:
    enabled: false
query_defaults:
  limit: 100
debug: false
`,
		},
	}

	// Fetch the existing ConfigMap
	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: desiredConfigMap.Name, Namespace: desiredConfigMap.Namespace}, existingConfigMap)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get ConfigMap: %w", err)
		}

		// Create the ConfigMap if it does not exist
		if err := r.Create(ctx, desiredConfigMap); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "CreateConfigMapFailed", fmt.Sprintf("Failed to create ConfigMap: %v", err))
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "CreatedConfigMap", fmt.Sprintf("Successfully created ConfigMap %s", desiredConfigMap.Name))
		return nil
	}

	// Compare the existing ConfigMap data with the desired data
	if !reflect.DeepEqual(existingConfigMap.Data, desiredConfigMap.Data) {
		// Update the existing ConfigMap's data
		existingConfigMap.Data = desiredConfigMap.Data

		// Update the ConfigMap in the cluster
		if err := r.Update(ctx, existingConfigMap); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdateConfigMapFailed", fmt.Sprintf("Failed to update ConfigMap: %v", err))
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdatedConfigMap", fmt.Sprintf("Successfully updated ConfigMap %s", existingConfigMap.Name))
	}

	return nil
}

func (r *vectorDbReconcilerImpl) ReconcileStatefulSet(ctx context.Context) error {
	labels := map[string]string{
		"app":        "weaviate",
		"deployment": r.genAIDeployment.Name,
	}

	// Define the name of the StatefulSet
	statefulSetName := "weaviate"
	namespace := r.genAIDeployment.Namespace
	configMapName := "weaviate-config"

	namespacedName := client.ObjectKey{Name: configMapName, Namespace: namespace}
	configMap := &corev1.ConfigMap{}

	// Get the ConfigMap
	err := r.Client.Get(ctx, namespacedName, configMap)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %v", err)
	}
	// Define the desired StatefulSet

	// Fetch the existing StatefulSet
	existingStatefulSet := &appsv1.StatefulSet{}
	err = r.Get(ctx, client.ObjectKey{Name: statefulSetName, Namespace: r.genAIDeployment.Namespace}, existingStatefulSet)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		// StatefulSet does not exist, so create it
		if err := r.Create(ctx, generateDesiredStatefulSet(r.genAIDeployment, configMap, labels)); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "CreateStatefulSetFailed", fmt.Sprintf("Failed to create StatefulSet: %v", err))
			return fmt.Errorf("failed to create StatefulSet: %w", err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "CreatedStatefulSet", fmt.Sprintf("Successfully created StatefulSet %s", statefulSetName))
		return nil
	}

	// Existing StatefulSet found - check for necessary updates
	updated := false

	// Iterate over containers and update only the one managed by the operator
	for i, container := range existingStatefulSet.Spec.Template.Spec.Containers {
		if container.Name == "weaviate" { // Only update the 'weaviate' container managed by this operator
			// Check and update the container's image
			if container.Image != r.genAIDeployment.Spec.VectorDbService.Image {
				existingStatefulSet.Spec.Template.Spec.Containers[i].Image = r.genAIDeployment.Spec.VectorDbService.Image
				updated = true
			}

			// Check and update environment variables, volume mounts, and ports
			existingStatefulSet.Spec.Template.Spec.Containers[i].Env = []corev1.EnvVar{
				{
					Name:  "AUTHENTICATION_APIKEY_ENABLED",
					Value: "false",
				},
				{
					Name:  "CLUSTER_DATA_BIND_PORT",
					Value: "7001",
				},
				{
					Name:  "CLUSTER_GOSSIP_BIND_PORT",
					Value: "7000",
				},
				{
					Name:  "GOGC",
					Value: "100",
				},
				{
					Name:  "PROMETHEUS_MONITORING_ENABLED",
					Value: "false",
				},
				{
					Name:  "PROMETHEUS_MONITORING_GROUP",
					Value: "false",
				},
				{
					Name:  "QUERY_MAXIMUM_RESULTS",
					Value: "100000",
				},
				{
					Name:  "REINDEX_VECTOR_DIMENSIONS_AT_STARTUP",
					Value: "false",
				},
				{
					Name:  "TRACK_VECTOR_DIMENSIONS",
					Value: "false",
				},
				{
					Name: "CLUSTER_BASIC_AUTH_USERNAME",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.genAIDeployment.Spec.VectorDbService.SecretRef.Name,
							},
							Key: "username",
						},
					},
				},
				{
					Name: "CLUSTER_BASIC_AUTH_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.genAIDeployment.Spec.VectorDbService.SecretRef.Name,
							},
							Key: "password",
						},
					},
				},
				{
					Name:  "PERSISTENCE_DATA_PATH",
					Value: "/var/lib/weaviate",
				},
				{
					Name:  "DEFAULT_VECTORIZER_MODULE",
					Value: "", // Set to an empty string if no default value
				},
				{
					Name:  "RAFT_JOIN",
					Value: "weaviate-0",
				},
				{
					Name:  "RAFT_BOOTSTRAP_EXPECT",
					Value: "1",
				},
			}

			existingStatefulSet.Spec.Template.Spec.Containers[i].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "weaviate-config",
					MountPath: "/weaviate-config",
				},
				{
					Name:      "weaviate-data",
					MountPath: "/var/lib/weaviate",
				},
			}

			existingStatefulSet.Spec.Template.Spec.Containers[i].Ports = []corev1.ContainerPort{
				{ContainerPort: 8080},
				{Name: "grpc", ContainerPort: 50051, Protocol: corev1.ProtocolTCP},
			}

			updated = true
		}
	}
	updated, err = r.CheckIfStatefulSetUpdateRequired(configMap, existingStatefulSet)
	if err != nil {
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdateStatefulSetFailed", fmt.Sprintf("Failed to get configmap for StatefulSet: %v", err))
		return err
	}

	if updated {
		// Update only the fields within spec.template
		if err := r.Update(ctx, existingStatefulSet); err != nil {
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdateStatefulSetFailed", fmt.Sprintf("Failed to update StatefulSet: %v", err))
			return fmt.Errorf("failed to update StatefulSet: %w", err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdatedStatefulSet", fmt.Sprintf("Successfully updated StatefulSet %s", existingStatefulSet.Name))
	} else {
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "ReconcileStatefulSet", "No changes detected, StatefulSet is up to date")
	}

	return nil
}

// Helper function to generate the desired StatefulSet
func generateDesiredStatefulSet(genAIDeployment *enterpriseApi.GenAIDeployment, configMap *corev1.ConfigMap, labels map[string]string) *appsv1.StatefulSet {
	trueValue := true
	runAsUser := int64(0)
	securityContext := &corev1.SecurityContext{
		RunAsUser:  &runAsUser,
		Privileged: &trueValue,
	}

	checksum, err := CalculateChecksum(configMap)
	if err != nil {
		//log.Error(err, "Failed to calculate checksum for ConfigMap")
		checksum = ""
	}
	serviceName := fmt.Sprintf("weaviate-headless.%s.svc.cluster.local", genAIDeployment.Namespace)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "weaviate",
			Namespace: genAIDeployment.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "weaviate",
				"app.kubernetes.io/managed-by": "sok",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &genAIDeployment.Spec.VectorDbService.Replicas,
			ServiceName: "weaviate-headless",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"checksum/confgig": checksum,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "configure-sysctl",
							SecurityContext: securityContext,
							Image:           "docker.io/alpine:latest",
							Command: []string{
								"sysctl",
								"-w",
								"vm.max_map_count=524288",
								"vm.overcommit_memory=1",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "weaviate",
							Image:   genAIDeployment.Spec.VectorDbService.Image,
							Command: []string{"/bin/weaviate"},
							Args: []string{
								"--host",
								"0.0.0.0",
								"--port",
								"8080",
								"--scheme",
								"http",
								"--config-file",
								"/weaviate-config/conf.yaml",
								"--read-timeout=60s",
								"--write-timeout=60s",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "AUTHENTICATION_APIKEY_ENABLED",
									Value: "false",
								},
								{
									Name:  "CLUSTER_DATA_BIND_PORT",
									Value: "7001",
								},
								{
									Name:  "CLUSTER_GOSSIP_BIND_PORT",
									Value: "7000",
								},
								{
									Name:  "GOGC",
									Value: "100",
								},
								{
									Name:  "PROMETHEUS_MONITORING_ENABLED",
									Value: "false",
								},
								{
									Name:  "PROMETHEUS_MONITORING_GROUP",
									Value: "false",
								},
								{
									Name:  "QUERY_MAXIMUM_RESULTS",
									Value: "100000",
								},
								{
									Name:  "REINDEX_VECTOR_DIMENSIONS_AT_STARTUP",
									Value: "false",
								},
								{
									Name:  "TRACK_VECTOR_DIMENSIONS",
									Value: "false",
								},
								{
									Name: "CLUSTER_BASIC_AUTH_USERNAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: genAIDeployment.Spec.VectorDbService.SecretRef.Name,
											},
											Key: "username",
										},
									},
								},
								{
									Name: "CLUSTER_BASIC_AUTH_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: genAIDeployment.Spec.VectorDbService.SecretRef.Name,
											},
											Key: "password",
										},
									},
								},
								{
									Name:  "PERSISTENCE_DATA_PATH",
									Value: "/var/lib/weaviate",
								},
								{
									Name:  "DEFAULT_VECTORIZER_MODULE",
									Value: "", // Set to an empty string if no default value
								},
								{
									Name:  "RAFT_JOIN",
									Value: "weaviate-0,weaviate-1,weaviate-2",
								},
								{
									Name:  "RAFT_BOOTSTRAP_EXPECT",
									Value: "3",
								},
								{
									Name:  "ENABLE_MODULES",
									Value: "text2vec-cohere,text2vec-huggingface,text2vec-openai,generative-openai,generative-cohere",
								},
								{
									Name:  "CLUSTER_JOIN",
									Value: serviceName,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "weaviate-config",
									MountPath: "/weaviate-config",
								},
								{
									Name:      "weaviate-data",
									MountPath: "/var/lib/weaviate",
								},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080},
								{Name: "grpc", ContainerPort: 50051, Protocol: corev1.ProtocolTCP},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/.well-known/live",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 900,
								PeriodSeconds:       10,
								TimeoutSeconds:      30,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/.well-known/ready",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 3,
								PeriodSeconds:       10,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "weaviate-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "weaviate-config",
									},
								},
							},
						},
						{
							Name: "weaviate-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "weaviate-data",
								},
							},
						},
					},
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 1,
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{"weaviate"},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "weaviate-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(genAIDeployment.Spec.VectorDbService.Storage.StorageCapacity),
							},
						},
						StorageClassName: &genAIDeployment.Spec.VectorDbService.Storage.StorageClassName,
					},
				},
			},
		},
	}
}

func (r *vectorDbReconcilerImpl) ReconcileServices(ctx context.Context) error {
	// Define the desired services: weaviate, weaviate-grpc, and weaviate-headless
	services := []*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "weaviate",
				Namespace: r.genAIDeployment.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "weaviate",
					"app.kubernetes.io/managed-by": "sok",
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Selector: map[string]string{
					"app": "weaviate",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "weaviate-grpc",
				Namespace: r.genAIDeployment.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "weaviate",
					"app.kubernetes.io/managed-by": "sok",
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Selector: map[string]string{
					"app": "weaviate",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "grpc",
						Port:       50051,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(50051),
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "weaviate-headless",
				Namespace: r.genAIDeployment.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":       "weaviate",
					"app.kubernetes.io/managed-by": "sok",
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "None", // Headless service
				Selector: map[string]string{
					"app": "weaviate",
				},
				Ports: []corev1.ServicePort{
					{
						Protocol:   corev1.ProtocolTCP,
						Port:       80,
						TargetPort: intstr.FromInt(7000),
					},
				},
				PublishNotReadyAddresses: true,
			},
		},
	}

	// Loop through each service and reconcile
	for _, service := range services {
		existingService := &corev1.Service{}
		err := r.Get(ctx, client.ObjectKey{Name: service.Name, Namespace: service.Namespace}, existingService)
		if err != nil {
			if client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to get existing Service %s: %w", service.Name, err)
			}
			// Service does not exist, so create it
			if err := r.Create(ctx, service); err != nil {
				return fmt.Errorf("failed to create Service %s: %w", service.Name, err)
			}
			r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "CreatedService", fmt.Sprintf("Successfully created Service %s", service.Name))
		} else {
			// If the service exists, do nothing
		}
	}

	r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "ReconcileServices", "Successfully reconciled Services")

	return nil
}

func (r *vectorDbReconcilerImpl) ReconcilePVC(ctx context.Context) error {
	// Get volume size and storage class from VectorDbService spec
	storageSize := r.genAIDeployment.Spec.VectorDbService.Storage.StorageCapacity
	storageClass := r.genAIDeployment.Spec.VectorDbService.Storage.StorageClassName

	// Define the desired PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "weaviate-data-pvc", // Predefined PVC name
			Namespace: r.genAIDeployment.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "weaviate",
				"app.kubernetes.io/managed-by": "sok",
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
			StorageClassName: &storageClass,
		},
	}

	// Check if the PVC already exists
	existingPVC := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{Name: pvc.Name, Namespace: pvc.Namespace}, existingPVC)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get existing PVC %s: %w", pvc.Name, err)
		}

		// PVC does not exist, so create it
		if err := r.Create(ctx, pvc); err != nil {
			return fmt.Errorf("failed to create PVC %s: %w", pvc.Name, err)
		}
		r.eventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "Created", fmt.Sprintf("Created PVC %s", pvc.Name))
		// FIXME
		return nil
	}

	// If the PVC exists, do nothing (or handle any potential updates if required)

	return nil
}

// CalculateChecksum calculates a SHA256 checksum for the given ConfigMap.
func CalculateChecksum(configMap *corev1.ConfigMap) (string, error) {
	if configMap == nil {
		return "", fmt.Errorf("ConfigMap is nil")
	}

	var sb strings.Builder
	keys := make([]string, 0, len(configMap.Data))
	for key := range configMap.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		sb.WriteString(key)
		sb.WriteString(configMap.Data[key])
	}

	hash := sha256.New()
	_, err := hash.Write([]byte(sb.String()))
	if err != nil {
		return "", fmt.Errorf("failed to compute checksum: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CheckIfStatefulSetUpdateRequired checks if the ConfigMap has changed and returns whether an update is required and the new StatefulSet.

func (r *vectorDbReconcilerImpl) CheckIfStatefulSetUpdateRequired(configMap *corev1.ConfigMap, statefulSet *appsv1.StatefulSet) (bool, error) {
	// Calculate the checksum of the ConfigMap
	newChecksum, err := CalculateChecksum(configMap)
	if err != nil {
		return false, fmt.Errorf("failed to calculate checksum: %v", err)
	}

	// Check the existing checksum in the StatefulSet template annotations
	existingChecksum, ok := statefulSet.Spec.Template.Annotations["checksum/config"]
	if !ok || existingChecksum != newChecksum {
		// Update the StatefulSet annotations and set update field to true
		if statefulSet.Spec.Template.Annotations == nil {
			statefulSet.Spec.Template.Annotations = make(map[string]string)
		}
		statefulSet.Spec.Template.Annotations["checksum/config"] = newChecksum
		// Assuming `update` is a custom annotation field to trigger the update
		statefulSet.Spec.Template.Annotations["update"] = "true"

		return true, nil
	}

	return false, nil
}
