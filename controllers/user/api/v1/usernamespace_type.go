/*
Copyright 2022 labring.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserNamespaceSpec defines the desired state of UserNamespace
type UserNamespaceSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Creator string `json:"creator"`
}

// UserNamespaceStatus defines the observed state of UserNamespace
type UserNamespaceStatus struct {
	// Phase is the recently observed lifecycle phase of user namespace
	//+kubebuilder:default:=Unknown
	//+kubebuilder:validation:Enum=Unknown;Pending;Ready
	Phase UserNamespacePhase `json:"phase,omitempty"`
	// Conditions contains the different condition statuses for this user namespace.
	Conditions []Condition `json:"conditions,omitempty"`
}

type UserNamespacePhase string

// These are the valid phases of node.
const (
	UserNamespaceUnknown UserNamespacePhase = "Unknown"
	UserNamespacePending UserNamespacePhase = "Pending"
	UserNamespaceReady   UserNamespacePhase = "Ready"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// UserNamespace is the Schema for the usernamespaces API
type UserNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserNamespaceSpec   `json:"spec,omitempty"`
	Status UserNamespaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserNamespaceList contains a list of UserNamespace
type UserNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserNamespace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserNamespace{}, &UserNamespaceList{})
}
