package genai

import (
	"context"
	"fmt"
	"reflect"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	enterpriseApi "github.com/vivekrsplunk/splunk-operator/api/v4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RayServiceReconciler is an interface for reconciling the RayService and its associated resources.
type RayServiceReconciler interface {
	Reconcile(ctx context.Context) error
	ReconcileRayCluster(ctx context.Context) error
	ReconcileConfigMap(ctx context.Context) error
	ReconcileSecret(ctx context.Context) error
	ReconcileService(ctx context.Context) error
}

// rayServiceReconcilerImpl is the concrete implementation of RayServiceReconciler.
type rayServiceReconcilerImpl struct {
	client.Client
	genAIDeployment *enterpriseApi.GenAIDeployment
	EventRecorder   record.EventRecorder
}

// NewRayServiceReconciler creates a new instance of RayServiceReconciler.
func NewRayServiceReconciler(c client.Client, genAIDeployment *enterpriseApi.GenAIDeployment, eventRecorder record.EventRecorder) RayServiceReconciler {
	return &rayServiceReconcilerImpl{
		Client:          c,
		genAIDeployment: genAIDeployment,
		EventRecorder:   eventRecorder,
	}
}

func (r *rayServiceReconcilerImpl) ReconcileRayCluster(ctx context.Context) error {
	log := log.FromContext(ctx)
	rayClusterName := fmt.Sprintf("%s-ray-cluster", r.genAIDeployment.Name)	
	// Fetch the existing RayCluster
	existingRayCluster := &rayv1.RayCluster{}
	err := r.Get(ctx, client.ObjectKey{Name: rayClusterName, Namespace: r.genAIDeployment.Namespace}, existingRayCluster)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get RayCluster")
			return err
		}

		// RayCluster does not exist, so create it
		desiredRayCluster := r.generateDesiredRayCluster()
		if err := r.Create(ctx, desiredRayCluster); err != nil {
			r.EventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "CreateRayClusterFailed", fmt.Sprintf("Failed to create RayCluster: %v", err))
			return fmt.Errorf("failed to create RayCluster: %w", err)
		}
		r.EventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "CreatedRayCluster", fmt.Sprintf("Successfully created RayCluster %s", desiredRayCluster.Name))
		return nil
	}

	// Check for differences in affinity and other fields
	updated := false

	// Set the Affinity only if it is provided in the spec
	if r.genAIDeployment.Spec.RayService.WorkerGroup.Affinity != nil {
		if !reflect.DeepEqual(existingRayCluster.Spec.WorkerGroupSpecs[0].Template.Spec.Affinity, r.genAIDeployment.Spec.RayService.WorkerGroup.Affinity) {
			existingRayCluster.Spec.WorkerGroupSpecs[0].Template.Spec.Affinity = r.genAIDeployment.Spec.RayService.WorkerGroup.Affinity
			updated = true
		}
	}

	// Update other fields if necessary...

	if updated {
		if err := r.Update(ctx, existingRayCluster); err != nil {
			r.EventRecorder.Event(r.genAIDeployment, corev1.EventTypeWarning, "UpdateRayClusterFailed", fmt.Sprintf("Failed to update RayCluster: %v", err))
			return fmt.Errorf("failed to update RayCluster: %w", err)
		}
		r.EventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "UpdatedRayCluster", fmt.Sprintf("Successfully updated RayCluster %s", existingRayCluster.Name))
	} else {
		r.EventRecorder.Event(r.genAIDeployment, corev1.EventTypeNormal, "ReconcileRayCluster", "No changes detected, RayCluster is up to date")
	}

	// Update the status based on the current status of RayCluster
	status := enterpriseApi.RayClusterStatus{}
	if existingRayCluster.Status.State == rayv1.AllPodRunningAndReadyFirstTime {
		status.State = "Running"
		status.Message = "RayService is running successfully"
	} else if existingRayCluster.Status.State == rayv1.RayClusterPodsProvisioning {
		status.State = "Provisioning"
		status.Message = "Provisioning RayCluster pods"
	} else {
		status.State = "Unknown"
		status.Message = "Unknown state of RayCluster"
	}

	return nil
}

// Helper function to generate the desired RayCluster
func (r *rayServiceReconcilerImpl) generateDesiredRayCluster() *rayv1.RayCluster {
	labels := map[string]string{
		"app":        "ray-service",
		"deployment": r.genAIDeployment.Name,
	}

	// Set default rayStartParams if not provided for HeadGroup
	if r.genAIDeployment.Spec.RayService.HeadGroup.RayStartParams == nil {
		r.genAIDeployment.Spec.RayService.HeadGroup.RayStartParams = map[string]string{
			"dashboard-host": "0.0.0.0", // Default value
		}
	}

	// Set default rayStartParams if not provided for WorkerGroup
	if r.genAIDeployment.Spec.RayService.WorkerGroup.RayStartParams == nil {
		r.genAIDeployment.Spec.RayService.WorkerGroup.RayStartParams = map[string]string{
			"num-cpus":            "4",          // Default value
			"object-store-memory": "1000000000", // Default value
		}
	}

	desiredRayCluster := &rayv1.RayCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ray-cluster", r.genAIDeployment.Name),
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Spec: rayv1.RayClusterSpec{
			HeadGroupSpec: rayv1.HeadGroupSpec{
				RayStartParams: r.genAIDeployment.Spec.RayService.HeadGroup.RayStartParams,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "ray-head",
								Image: r.genAIDeployment.Spec.RayService.Image,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse(r.genAIDeployment.Spec.RayService.HeadGroup.NumCpus),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
								},
							},
						},
					},
				},
			},
			WorkerGroupSpecs: []rayv1.WorkerGroupSpec{
				{
					RayStartParams: r.genAIDeployment.Spec.RayService.WorkerGroup.RayStartParams,
					Replicas:       &r.genAIDeployment.Spec.RayService.WorkerGroup.Replicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							Affinity: r.genAIDeployment.Spec.RayService.WorkerGroup.Affinity,
							Containers: []corev1.Container{
								{
									Name:      "ray-worker",
									Image:     r.genAIDeployment.Spec.RayService.Image,
									Resources: r.genAIDeployment.Spec.RayService.WorkerGroup.Resources,
								},
							},
						},
					},
				},
			},
		},
	}
	return desiredRayCluster
}

func (r *rayServiceReconcilerImpl) ReconcileConfigMap(ctx context.Context) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ray-service-config",
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Data: map[string]string{
			"rayConfigKey": "rayConfigValue",
		},
	}

	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
	}
	return nil
}

func (r *rayServiceReconcilerImpl) ReconcileSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ray-service-secret",
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Data: map[string][]byte{
			"authKey": []byte("ray-sais-sok-secret-auth-key"),
		},
	}

	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, existingSecret)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := r.Create(ctx, secret); err != nil {
			return fmt.Errorf("failed to create Secret: %w", err)
		}
	}
	return nil
}

func (r *rayServiceReconcilerImpl) ReconcileService(ctx context.Context) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ray-service", r.genAIDeployment.Name),
			Namespace: r.genAIDeployment.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.genAIDeployment, enterpriseApi.GroupVersion.WithKind("GenAIDeployment")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":        "ray-service",
				"deployment": r.genAIDeployment.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:     6379,
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

// Reconcile manages the complete reconciliation logic for the RayService and returns its status.
func (r *rayServiceReconcilerImpl) Reconcile(ctx context.Context) error {
	status := enterpriseApi.RayClusterStatus{}

	// Reconcile the RayCluster resource for RayService
	if err := r.ReconcileRayCluster(ctx); err != nil {
		status.State = "Error"
		status.Message = "Failed to reconcile RayCluster"
		return err
	}

	// Reconcile the ConfigMap for RayService if needed
	if err := r.ReconcileConfigMap(ctx); err != nil {
		status.State = "Error"
		status.Message = "Failed to reconcile ConfigMap"
		return err
	}

	// Reconcile the Secret for RayService if needed
	if err := r.ReconcileSecret(ctx); err != nil {
		status.State = "Error"
		status.Message = "Failed to reconcile Secret"
		return err
	}

	// Reconcile the Service for RayService
	if err := r.ReconcileService(ctx); err != nil {
		status.State = "Error"
		status.Message = "Failed to reconcile Service"
		return err
	}

	// If all reconciliations succeed, update the status to Running
	status.State = "Running"
	status.Message = "RayService is running successfully"
	return nil
}
