/*
Copyright 2021.

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

package enterprise

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"strings"
	//promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	enterpriseApi "github.com/vivekrsplunk/splunk-operator/api/v4"
	splcommon "github.com/vivekrsplunk/splunk-operator/pkg/splunk/common"
	genai "github.com/vivekrsplunk/splunk-operator/pkg/splunk/genai"

	//"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// GenAIDeploymentReconciler reconciles a GenAIDeployment object
type GenAIDeploymentReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
}

// ApplyGenAIDeployment reconciles the state of a Splunk Enterprise cluster manager.
func ApplyGenAIDeployment(ctx context.Context, client splcommon.ControllerClient, eventRecorder record.EventRecorder, cr *enterpriseApi.GenAIDeployment) (reconcile.Result, error) {
	g := &GenAIDeploymentReconciler{Client: client, Scheme: client.Scheme(), eventRecorder: eventRecorder}
	return g.Reconcile(ctx, cr)
}

// Reconcile is the main reconciliation loop for GenAIDeployment.
func (r *GenAIDeploymentReconciler) Reconcile(ctx context.Context, cr *enterpriseApi.GenAIDeployment) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Initialize reconcilers for each service
	saisReconciler := genai.NewSaisServiceReconciler(r.Client, cr, r.eventRecorder)
	vectorDbReconciler := genai.NewVectorDbReconciler(r.Client, cr, r.eventRecorder)
	rayReconciler := genai.NewRayServiceReconciler(r.Client, cr, r.eventRecorder)
	prometheusRules := genai.NewPrometheusRuleReconciler(r.Client, cr, r.eventRecorder)

	// Call ValidateSpec before reconciling services
	if err := r.ValidateSpec(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile PrometheusRules

	// Reconcile SaisService
	err := saisReconciler.Reconcile(ctx)
	if err != nil {
		log.Error(err, "Failed to reconcile SaisService")
		if updateErr := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeSaisService, metav1.ConditionFalse, "ReconcileFailed", err.Error()); updateErr != nil {
			log.Error(updateErr, "Failed to update SaisService condition after reconcile failure")
		}
		return ctrl.Result{}, err
	}
	if err := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeSaisService, metav1.ConditionTrue, "Reconciled", "SaisService is running and healthy"); err != nil {
		log.Error(err, "Failed to update SaisService condition")
		return ctrl.Result{}, err
	}

	// Reconcile VectorDbService
	err = vectorDbReconciler.Reconcile(ctx)
	if err != nil {
		log.Error(err, "Failed to reconcile VectorDbService")
		if updateErr := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeVectorDbService, metav1.ConditionFalse, "ReconcileFailed", err.Error()); updateErr != nil {
			log.Error(updateErr, "Failed to update VectorDbService condition after reconcile failure")
		}
		return ctrl.Result{}, err
	}
	if err := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeVectorDbService, metav1.ConditionTrue, "Reconciled", "VectorDbService is running and healthy"); err != nil {
		log.Error(err, "Failed to update VectorDbService condition")
		return ctrl.Result{}, err
	}

	// Reconcile RayService
	err = rayReconciler.Reconcile(ctx)
	if err != nil {
		log.Error(err, "Failed to reconcile RayService")
		if updateErr := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeRayService, metav1.ConditionFalse, "ReconcileFailed", err.Error()); updateErr != nil {
			log.Error(updateErr, "Failed to update RayService condition after reconcile failure")
		}
		return ctrl.Result{}, err
	}
	if err := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeRayService, metav1.ConditionTrue, "Reconciled", "RayService is running and healthy"); err != nil {
		log.Error(err, "Failed to update RayService condition")
		return ctrl.Result{}, err
	}

	prometheusRules.Reconcile(ctx)

	// Update overall workload condition
	if err := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeGenAIWorkload, metav1.ConditionTrue, "AllComponentsHealthy", "All components of GenAI workload are running and healthy"); err != nil {
		log.Error(err, "Failed to update overall GenAI workload condition")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled GenAIDeployment")
	return ctrl.Result{}, nil
}

func (r *GenAIDeploymentReconciler) updateGenAIDeploymentCondition(ctx context.Context, deployment *enterpriseApi.GenAIDeployment, conditionType string, status metav1.ConditionStatus, reason, message string) error {
	// Find the existing condition
	existingConditionIndex := -1
	for i, cond := range deployment.Status.Conditions {
		if cond.Type == conditionType {
			existingConditionIndex = i
			break
		}
	}

	// Create a new condition
	newCondition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	if existingConditionIndex == -1 {
		// Condition does not exist, add it
		deployment.Status.Conditions = append(deployment.Status.Conditions, newCondition)
	} else {
		// Condition exists, update it
		deployment.Status.Conditions[existingConditionIndex] = newCondition
	}

	// Update the status in Kubernetes
	if err := r.Status().Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update GenAIDeployment status: %w", err)
	}
	return nil
}

// ValidateSpec validates the GenAIDeployment spec and ensures all required fields are set.
func (r *GenAIDeploymentReconciler) ValidateSpec(ctx context.Context, cr *enterpriseApi.GenAIDeployment) error {
	log := log.FromContext(ctx)

	// Initialize a variable to track validation errors
	var validationErrors []string

	// 1. Validate Service Account
	if cr.Spec.ServiceAccount != "" {
		serviceAccount := &corev1.ServiceAccount{}
		err := r.Get(ctx, client.ObjectKey{Name: cr.Spec.ServiceAccount, Namespace: cr.Namespace}, serviceAccount)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Error(err, "ServiceAccount not found", "ServiceAccountName", cr.Spec.ServiceAccount)
				if updateErr := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeServiceAccount, metav1.ConditionFalse, "NotFound", "ServiceAccount not found"); updateErr != nil {
					log.Error(updateErr, "Failed to update ServiceAccount condition after not found")
				}
				validationErrors = append(validationErrors, fmt.Sprintf("service account %s not found", cr.Spec.ServiceAccount))
			} else {
				log.Error(err, "Failed to get ServiceAccount", "ServiceAccountName", cr.Spec.ServiceAccount)
				return err
			}
		}
	} else {
		validationErrors = append(validationErrors, "serviceAccount is required")
	}

	// 2. Validate SaisServiceSpec
	sais := cr.Spec.SaisService
	if sais.Image == "" {
		validationErrors = append(validationErrors, "saisService.image is required")
	}
	if sais.Replicas < 0 {
		validationErrors = append(validationErrors, "saisService.replicas must be non-negative")
	}
	// Additional validations can be added as needed, e.g., validate Resources
	if err := validateResourceRequirements(sais.Resources); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("saisService.resources: %v", err))
	}
	if sais.SchedulerName == "" {
		validationErrors = append(validationErrors, "saisService.schedulerName is required")
	}
	// Optionally, validate Affinity, Tolerations, etc., if specific rules are needed

	// 3. Validate Bucket (VolumeSpec)
	if err := validateVolumeSpec(cr.Spec.Bucket); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("bucket: %v", err))
	}

	// 4. Validate RayServiceSpec if enabled
	if cr.Spec.RayService.Enabled {
		ray := cr.Spec.RayService
		if ray.Image == "" {
			validationErrors = append(validationErrors, "rayService.image is required when RayService is enabled")
		}
		/*
			if ray.HeadGroup.Resources == nil {
				validationErrors = append(validationErrors, "rayService.headGroup.resources is required")
			} else {
				if err := validateResourceRequirements(ray.HeadGroup.Resources); err != nil {
					validationErrors = append(validationErrors, fmt.Sprintf("rayService.headGroup.resources: %v", err))
				}
			}
			// Validate RayStartParams if necessary
			if ray.WorkerGroup.Resources == nil {
				validationErrors = append(validationErrors, "rayService.workerGroup.resources is required")
			} else {
				if err := validateResourceRequirements(ray.WorkerGroup.Resources); err != nil {
					validationErrors = append(validationErrors, fmt.Sprintf("rayService.workerGroup.resources: %v", err))
				}
			}
		*/
		// Additional validations for Affinity, etc., can be added here
	}

	// 5. Validate VectorDbServiceSpec if enabled
	if cr.Spec.VectorDbService.Enabled {
		vdb := cr.Spec.VectorDbService
		if vdb.Image == "" {
			validationErrors = append(validationErrors, "vectordbService.image is required when VectorDbService is enabled")
		}
		if vdb.SecretRef.Name == "" {
			validationErrors = append(validationErrors, "vectordbService.secretRef.name is required when VectorDbService is enabled")
		}
		if err := validateResourceRequirements(vdb.Resources); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("vectordbService.resources: %v", err))
		}
		// Validate Storage if necessary
		if err := validateStorageClassSpec(vdb.Storage); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("vectordbService.storage: %v", err))
		}
		if vdb.Replicas < 0 {
			validationErrors = append(validationErrors, "vectordbService.replicas must be non-negative")
		}
		// Additional validations for Affinity, Tolerations, etc., can be added here
	}

	// 6. Validate Prometheus Operator Configuration
	if cr.Spec.PrometheusOperator {
		if cr.Spec.PrometheusRuleConfig == "" {
			validationErrors = append(validationErrors, "prometheusRuleConfig is required when prometheusOperator is enabled")
		} else {
			// Optionally, check if the ConfigMap exists
			configMap := &corev1.ConfigMap{}
			err := r.Get(ctx, client.ObjectKey{Name: cr.Spec.PrometheusRuleConfig, Namespace: cr.Namespace}, configMap)
			if err != nil {
				if errors.IsNotFound(err) {
					validationErrors = append(validationErrors, fmt.Sprintf("prometheusRuleConfig ConfigMap %s not found", cr.Spec.PrometheusRuleConfig))
				} else {
					log.Error(err, "Failed to get PrometheusRuleConfig ConfigMap", "ConfigMapName", cr.Spec.PrometheusRuleConfig)
					return err
				}
			}
		}
	}

	// 7. Validate GPU Requirements
	if cr.Spec.RequireGPU {
		if cr.Spec.GPUResource == "" {
			validationErrors = append(validationErrors, "gpuResource is required when requireGPU is set to true")
		} else {
			// Optionally, validate if the GPU resource type is supported
			validGPUResources := map[string]bool{
				"nvidia.com/gpu": true,
				// Add other supported GPU resource types here
			}
			if !validGPUResources[cr.Spec.GPUResource] {
				validationErrors = append(validationErrors, fmt.Sprintf("gpuResource %s is not supported", cr.Spec.GPUResource))
			}
		}
	}

	// 8. Validate TopologySpreadConstraints and Affinity if necessary
	// (Optional: Add specific validations based on your requirements)

	// 9. Aggregate Validation Errors
	if len(validationErrors) > 0 {
		// Update SpecValidation condition to False with aggregated errors
		errMsg := strings.Join(validationErrors, "; ")
		if updateErr := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeSpecValidation, metav1.ConditionFalse, "InvalidSpec", errMsg); updateErr != nil {
			log.Error(updateErr, "Failed to update SpecValidation condition")
			return updateErr
		}
		return fmt.Errorf("spec validation failed: %s", errMsg)
	}

	// If all validations pass, update the condition to true
	if err := r.updateGenAIDeploymentCondition(ctx, cr, enterpriseApi.ConditionTypeSpecValidation, metav1.ConditionTrue, "Validated", "Spec is valid"); err != nil {
		log.Error(err, "Failed to update SpecValidation condition")
		return err
	}

	return nil
}

// validateResourceRequirements checks if the resource requirements are properly set.
func validateResourceRequirements(resources corev1.ResourceRequirements) error {
	if resources.Requests == nil || resources.Limits == nil {
		return fmt.Errorf("both requests and limits must be specified")
	}
	// Example: Ensure CPU and Memory requests and limits are set
	if _, ok := resources.Requests[corev1.ResourceCPU]; !ok {
		return fmt.Errorf("cpu request is required")
	}
	if _, ok := resources.Requests[corev1.ResourceMemory]; !ok {
		return fmt.Errorf("memory request is required")
	}
	if _, ok := resources.Limits[corev1.ResourceCPU]; !ok {
		return fmt.Errorf("cpu limit is required")
	}
	if _, ok := resources.Limits[corev1.ResourceMemory]; !ok {
		return fmt.Errorf("memory limit is required")
	}
	return nil
}

// validateVolumeSpec checks if the VolumeSpec is properly configured.
// Implement this function based on the actual structure and requirements of VolumeSpec.
func validateVolumeSpec(volume enterpriseApi.VolumeSpec) error {
	// Placeholder implementation: Ensure at least one storage type is specified
	if volume.Provider == "" && volume.Endpoint == "" {
		return fmt.Errorf("either Provider or Endpoint must be specified")
	}
	return nil
}

// validateStorageClassSpec checks if the StorageCapacity is properly configured.
// Implement this function based on the actual structure and requirements of StorageClassSpec.
func validateStorageClassSpec(storage enterpriseApi.StorageClassSpec) error {
	// Placeholder implementation: Ensure StorageCapacity name is provided
	if storage.StorageCapacity == "" {
		return fmt.Errorf("storage.StorageCapacity is required")
	}
	// Add more validations as needed
	return nil
}
