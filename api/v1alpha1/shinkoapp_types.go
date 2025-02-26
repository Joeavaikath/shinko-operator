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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ShinkoAppSpec defines the desired state of ShinkoApp.
type ShinkoAppSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	AppImage string `json:"appImage"`

	Replicas int32 `json:"replicas,omitempty"`

	AutoUpdate bool `json:"autoUpdate,omitempty"`
}

// ShinkoAppStatus defines the observed state of ShinkoApp.
type ShinkoAppStatus struct {
	// Latest deployed app image
	CurrentAppImage string `json:"currentAppImage,omitempty"`

	// Current observed replica count
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Last Quay image check timestamp
	LastImageCheck string `json:"lastImageCheck,omitempty"`

	// Last successful backup timestamp
	LastBackup string `json:"lastBackup,omitempty"`

	// List of existing backups managed by this CRD
	BackupHistory []string `json:"backupHistory,omitempty"`

	// Conditions to track app readiness/errors
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ShinkoApp is the Schema for the shinkoapps API.
type ShinkoApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShinkoAppSpec   `json:"spec,omitempty"`
	Status ShinkoAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ShinkoAppList contains a list of ShinkoApp.
type ShinkoAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShinkoApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShinkoApp{}, &ShinkoAppList{})
}
