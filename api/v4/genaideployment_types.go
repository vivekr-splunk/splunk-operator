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

// +kubebuilder:validation:Pattern=^[a-zA-Z0-9.-]+$
const (
	// GenAIDeploymentPausedAnnotation is the annotation that pauses the reconciliation (triggers
	// an immediate requeue)
	GenAIDeploymentPausedAnnotation = "genaideployment.enterprise.splunk.com/paused"
)

// GenAIDeploymentSpec defines the desired state of GenAIDeployment
type GenAIDeploymentSpec struct {
	// Configuration for SaisService
	// +kubebuilder:validation:Required
	SaisService SaisServiceSpec `json:"saisService"`

	// S3 bucket or other remote storage configuration
	// +kubebuilder:validation:Required
	Bucket VolumeSpec `json:"bucket"`

	// Service Account for IRSA authentication
	// +kubebuilder:validation:Required
	ServiceAccount string `json:"serviceAccount"`

	// Ray service configuration with RayServiceSpec
	// +kubebuilder:validation:Optional
	RayService RayServiceSpec `json:"rayService,omitempty"`

	// VectorDB service configuration
	// +kubebuilder:validation:Optional
	VectorDbService VectorDbSpec `json:"vectordbService,omitempty"`

	// Indicates if Prometheus Operator is installed
	// +kubebuilder:validation:Required
	PrometheusOperator bool `json:"prometheusOperator"`

	// Name of the ConfigMap containing Prometheus rules
	// +kubebuilder:validation:Optional
	PrometheusRuleConfig string `json:"prometheusRuleConfig,omitempty"`

	// Indicates if GPU support is required for the workload
	// +kubebuilder:validation:Required
	RequireGPU bool `json:"requireGPU"`

	// Specific GPU resource type (e.g., "nvidia.com/gpu")
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=nvidia.com/gpu;amd.com/gpu
	GPUResource string `json:"gpuResource,omitempty"`
}

// SaisServiceSpec defines the configuration for a single SaisService deployment
type SaisServiceSpec struct {
	// Secret reference for sensitive data
	// +kubebuilder:validation:Optional
	SecretRef corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// ConfigMap reference for environment variables
	// +kubebuilder:validation:Optional
	ConfigMapRef corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// Container image for the service
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Resource requirements for the container (CPU, Memory)
	// +kubebuilder:validation:Required
	Resources corev1.ResourceRequirements `json:"resources"`

	// Volume specifications using corev1.Volume
	// +kubebuilder:validation:Optional
	Volume corev1.Volume `json:"volume,omitempty"`

	// Number of replicas for the service
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas"`

	// Name of Scheduler to use for pod placement (defaults to “default-scheduler”)
	// +kubebuilder:validation:Required
	SchedulerName string `json:"schedulerName"`

	// Kubernetes Affinity rules that control how pods are assigned to particular nodes.
	// +kubebuilder:validation:Required
	Affinity corev1.Affinity `json:"affinity"`

	// Pod's tolerations for Kubernetes node's taint
	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// ServiceTemplate is a template used to create Kubernetes services
	// +kubebuilder:validation:Required
	ServiceTemplate corev1.Service `json:"serviceTemplate"`

	// TopologySpreadConstraint https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
	// +kubebuilder:validation:Optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// HostAliases for the pods
	// +kubebuilder:validation:Optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`
}

// HeadGroup defines the head group configuration for RayService
type HeadGroup struct {
	// Resource requirements for the container (CPU, Memory)
	// +kubebuilder:validation:Required
	Resources corev1.ResourceRequirements `json:"resources"`

	// Number of CPUs allocated
	// +kubebuilder:validation:Optional
	NumCpus string `json:"numCpus,omitempty"`

	// Parameters for the head group
	// +kubebuilder:validation:Optional
	RayStartParams map[string]string `json:"rayStartParams,omitempty"`
}

// WorkerGroup defines the worker group configuration for RayService
type WorkerGroup struct {
	// Resource requirements for the container (CPU, Memory)
	// +kubebuilder:validation:Required
	Resources corev1.ResourceRequirements `json:"resources"`

	// Number of replicas
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	Replicas int32 `json:"replicas,omitempty"`

	// Parameters for the worker group
	// +kubebuilder:validation:Optional
	RayStartParams map[string]string `json:"rayStartParams,omitempty"`

	// Kubernetes Affinity rules that control how pods are assigned to particular nodes.
	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// RayServiceSpec defines the Ray service configuration
type RayServiceSpec struct {
	// Whether RayService is enabled
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Additional RayService configuration
	// +kubebuilder:validation:Optional
	Config string `json:"config,omitempty"`

	// Container image for the Ray service
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`

	// Head group configuration
	// +kubebuilder:validation:Optional
	HeadGroup HeadGroup `json:"headGroup,omitempty"`

	// Worker group configuration
	// +kubebuilder:validation:Optional
	WorkerGroup WorkerGroup `json:"workerGroup,omitempty"`
}

// VectorDbSpec defines the VectorDB service configuration
type VectorDbSpec struct {
	// Container image for the service
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Whether VectorDBService is enabled
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Reference to a user-provided secret
	// +kubebuilder:validation:Required
	SecretRef corev1.LocalObjectReference `json:"secretRef"`

	// Additional VectorDB configuration
	// +kubebuilder:validation:Optional
	Config string `json:"config,omitempty"`

	// Resource requirements for the container (CPU, Memory)
	// +kubebuilder:validation:Required
	Resources corev1.ResourceRequirements `json:"resources"`

	// Storage configuration for persistent or ephemeral storage
	// +kubebuilder:validation:Required
	Storage StorageClassSpec `json:"storage"`

	// Number of replicas for the service
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas"`

	// Kubernetes Affinity rules that control how pods are assigned to particular nodes.
	// +kubebuilder:validation:Required
	Affinity corev1.Affinity `json:"affinity"`

	// Pod's tolerations for Kubernetes node's taint
	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// ServiceTemplate is a template used to create Kubernetes services
	// +kubebuilder:validation:Required
	ServiceTemplate corev1.Service `json:"serviceTemplate"`

	// TopologySpreadConstraint https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/
	// +kubebuilder:validation:Optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
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

	ConditionTypeServiceAccount = "ServiceAccount"

	ConditionTypeSpecValidation = "SpecValidation"
)

// GenAIDeploymentStatus defines the observed state of GenAIDeployment
type GenAIDeploymentStatus struct {
	// Conditions store the status conditions of the GenAIDeployment instances
	// +kubebuilder:validation:Optional
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
	// Name of the SaisService deployment
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Number of replicas running
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas"`

	// Current status of the deployment (e.g., "Running", "Failed")
	// +kubebuilder:validation:Required
	Status string `json:"status"`

	// Additional information about the service status
	// +kubebuilder:validation:Optional
	Message string `json:"message"`
}

// VectorDbStatus defines the status of the VectorDB service
type VectorDbStatus struct {
	// Whether the VectorDB service is enabled
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Current status of the VectorDB service (e.g., "Running", "Failed")
	// +kubebuilder:validation:Required
	Status string `json:"status"`

	// Additional information about the VectorDB service status
	// +kubebuilder:validation:Optional
	Message string `json:"message"`
}

// RayClusterStatus defines the status of the RayCluster
type RayClusterStatus struct {
	// Name of the RayCluster
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName"`

	// Current state of the RayCluster (e.g., "Running", "Failed")
	// +kubebuilder:validation:Required
	State string `json:"state"`

	// Conditions describing the state of the RayCluster
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Additional information about the RayCluster status
	// +kubebuilder:validation:Optional
	Message string `json:"message"`
}

// ClusterManager is the Schema for the cluster manager API
// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=genaideployments,scope=Namespaced,shortName=genai-splunk
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Phase of the genai deployment"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message",description="Auxiliary message describing CR status"
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
