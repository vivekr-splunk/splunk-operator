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

package v4

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GenAIDeploymentPausedAnnotation is the annotation that pauses the reconciliation (triggers
	// an immediate requeue)
	GenAIDeploymentPausedAnnotation = "genaideployment.enterprise.splunk.com/paused"
)

// GenAIDeploymentSpec defines the desired state of GenAIDeployment
type GenAIDeploymentSpec struct {
	SaisService          SaisServiceSpec `json:"saisService"`                    // Configuration for SaisService
	Bucket               VolumeSpec      `json:"bucket"`                         // S3 bucket or other remote storage configuration
	ServiceAccount       string          `json:"serviceAccount,omitempty"`       // Service Account for IRSA authentication
	RayService           RayServiceSpec  `json:"rayService,omitempty"`           // Ray service configuration with RayServiceSpec
	VectorDbService      VectorDbSpec    `json:"vectordbService,omitempty"`      // VectorDB service configuration
	PrometheusOperator   bool            `json:"prometheusOperator"`             // Indicates if Prometheus Operator is installed
	PrometheusRuleConfig string          `json:"prometheusRuleConfig,omitempty"` // Name of the ConfigMap containing Prometheus rules
	RequireGPU           bool            `json:"requireGPU,omitempty"`           // Indicates if GPU support is required for the workload
	GPUResource          string          `json:"gpuResource,omitempty"`          // Specific GPU resource type (e.g., "nvidia.com/gpu")
}

// SaisServiceSpec defines the configuration for a single SaisService deployment
type SaisServiceSpec struct {
	SecretRef                 corev1.LocalObjectReference       `json:"secretRef,omitempty"`                 // Secret reference for sensitive data
	ConfigMapRef              corev1.LocalObjectReference       `json:"configMapRef,omitempty"`              // ConfigMap reference for environment variables
	Image                     string                            `json:"image"`                               // Container image for the service
	Resources                 corev1.ResourceRequirements       `json:"resources"`                           // Resource requirements for the container (CPU, Memory)
	Volume                    corev1.Volume                     `json:"volume,omitempty"`                    // Volume specifications using corev1.Volume
	Replicas                  int32                             `json:"replicas"`                            // Number of replicas for the service
	SchedulerName             string                            `json:"schedulerName"`                       // Name of Scheduler to use for pod placement (defaults to “default-scheduler”)
	Affinity                  corev1.Affinity                   `json:"affinity"`                            // Kubernetes Affinity rules that control how pods are assigned to particular nodes.
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty"`               // Pod's tolerations for Kubernetes node's taint
	ServiceTemplate           corev1.Service                    `json:"serviceTemplate"`                     // ServiceTemplate is a template used to create Kubernetes services
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"` // TopologySpreadConstraint https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
}

type HeadGroup struct {
	Resources      corev1.ResourceRequirements `json:"resources"` // Resource requirements for the container (CPU, Memory)
	NumCpus        string                      `json:"numCpus,omitempty"`
	RayStartParams map[string]string           `json:"rayStartParams,omitempty"` // Parameters for the head group
}

type WorkerGroup struct {
	Resources      corev1.ResourceRequirements `json:"resources"` // Resource requirements for the container (CPU, Memory)
	Replicas       int32                       `json:"replicas,omitempty"`
	RayStartParams map[string]string           `json:"rayStartParams,omitempty"` // Parameters for the worker group
	Affinity                  *corev1.Affinity                   `json:"affinity"`                            // Kubernetes Affinity rules that control how pods are assigned to particular nodes.
}

// RayServiceSpec defines the Ray service configuration
type RayServiceSpec struct {
	Enabled     bool        `json:"enabled"`          // Whether RayService is enabled
	Config      string      `json:"config,omitempty"` // Additional RayService configuration
	Image       string      `json:"image,omitempty"`
	HeadGroup   HeadGroup   `json:"headGroup,omitempty"`
	WorkerGroup WorkerGroup `json:"workerGroup,omitempty"`
}

// VectorDbSpec defines the VectorDB service configuration
type VectorDbSpec struct {
	Image                     string                            `json:"image"`                               // Container image for the service
	Enabled                   bool                              `json:"enabled"`                             // Whether VectorDBService is enabled
	SecretRef                 corev1.LocalObjectReference       `json:"secretRef"`                           // Reference to a user-provided secret
	Config                    string                            `json:"config,omitempty"`                    // Additional VectorDB configuration
	Resources                 corev1.ResourceRequirements       `json:"resources"`                           // Resource requirements for the container (CPU, Memory)
	Storage                   StorageClassSpec                  `json:"storage"`                             // Storage configuration for persistent or ephemeral storage
	Replicas                  int32                             `json:"replicas"`                            // Number of replicas for the service
	Affinity                  corev1.Affinity                   `json:"affinity"`                            // Kubernetes Affinity rules that control how pods are assigned to particular nodes.
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty"`               // Pod's tolerations for Kubernetes node's taint
	ServiceTemplate           corev1.Service                    `json:"serviceTemplate"`                     // ServiceTemplate is a template used to create Kubernetes services
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"` // TopologySpreadConstraint https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
}

const (
	// ConditionTypeSaisService represents the condition type for SaisService
	ConditionTypeSaisService = "SaisService"

	// ConditionTypeVectorDbService represents the condition type for VectorDbService
	ConditionTypeVectorDbService = "VectorDbService"

	// ConditionTypeRayService represents the condition type for RayService
	ConditionTypeRayService = "RayService"

	// ConditionTypeGenAIWorkload represents the overall condition of the GenAI workload
	ConditionTypeGenAIWorkload = "GenAIWorkload"
)

// GenAIDeploymentStatus defines the observed state of GenAIDeployment
type GenAIDeploymentStatus struct {
	//SaisServiceStatus   SaisServiceStatus `json:"saisServiceStatus,omitempty"`   // Status of the SaisService
	//VectorDbStatus      VectorDbStatus    `json:"vectorDbStatus,omitempty"`      // Status of the VectorDB service
	//RayClusterStatus    RayClusterStatus  `json:"rayClusterStatus,omitempty"`    // Status of the RayCluster
	//GenAIWorkloadStatus string            `json:"genAIWorkloadStatus,omitempty"` // Overall status of the GenAI workloa// Conditions store the status conditions of the GenAIDeployment instances
	// Represents the observations of a GenAIDeployment's current state.
	// GenAIDeployment.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// GenAIDeployment.status.conditions.status are one of True, False, Unknown.
	// GenAIDeployment.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// GenAIDeployment.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// SaisServiceStatus defines the status of the SaisService deployment
type SaisServiceStatus struct {
	Name     string `json:"name"`     // Name of the SaisService deployment
	Replicas int32  `json:"replicas"` // Number of replicas running
	Status   string `json:"status"`   // Current status of the deployment (e.g., "Running", "Failed")
	Message  string `json:"message"`  // Additional information about the service status
}

// VectorDbStatus defines the status of the VectorDB service
type VectorDbStatus struct {
	Enabled bool   `json:"enabled"` // Whether the VectorDB service is enabled
	Status  string `json:"status"`  // Current status of the VectorDB service (e.g., "Running", "Failed")
	Message string `json:"message"` // Additional information about the VectorDB service status
}

// RayClusterStatus defines the status of the RayCluster
type RayClusterStatus struct {
	ClusterName string             `json:"clusterName"`          // Name of the RayCluster
	State       string             `json:"state"`                // Current state of the RayCluster (e.g., "Running", "Failed")
	Conditions  []metav1.Condition `json:"conditions,omitempty"` // Conditions describing the state of the RayCluster
	Message     string             `json:"message"`              // Additional information about the RayCluster status
}

// ClusterManager is the Schema for the cluster manager API
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=genaideployments,scope=Namespaced,shortName=genai-splunk
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Phase of the genai deployment"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message",description="Auxillary message describing CR status"
// +kubebuilder:storageversion
// GenAIDeployment is the Schema for the genaideployments API
type GenAIDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GenAIDeploymentSpec   `json:"spec,omitempty"`
	Status GenAIDeploymentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GenAIDeploymentList contains a list of GenAIDeployment
type GenAIDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GenAIDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GenAIDeployment{}, &GenAIDeploymentList{})
}

// NewEvent creates a new event associated with the object and ready
// to be published to the kubernetes API.
func (shcstr *GenAIDeployment) NewEvent(eventType, reason, message string) corev1.Event {
	t := metav1.Now()
	return corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: reason + "-",
			Namespace:    shcstr.ObjectMeta.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       "GenAIDeployment",
			Namespace:  shcstr.Namespace,
			Name:       shcstr.Name,
			UID:        shcstr.UID,
			APIVersion: GroupVersion.String(),
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: "splunk-genaideployer-controller",
		},
		FirstTimestamp:      t,
		LastTimestamp:       t,
		Count:               1,
		Type:                eventType,
		ReportingController: "enterprise.splunk.com/genaideployer-controller",
	}
}
