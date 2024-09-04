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

	//promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	enterpriseApi "github.com/vivekrsplunk/splunk-operator/api/v4"
	splcommon "github.com/vivekrsplunk/splunk-operator/internal/pkg/splunk/common"
	genai "github.com/vivekrsplunk/splunk-operator/internal/pkg/splunk/genai"

	//"gopkg.in/yaml.v2"
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
