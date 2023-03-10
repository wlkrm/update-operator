/*
Copyright 2022.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RealTimeThreadSpec is the description of a real time thread
type RealTimeThreadSpec struct {
	// Name is the name of the thread
	Name        string `json:"name,omitempty"`
	Rt_runtime  string `json:"rt_runtime,omitempty"`
	Rt_period   string `json:"rt_period,omitempty"`
	Rt_wcet     string `json:"rt_wcet,omitempty"`
	Rt_deadline string `json:"rt_deadline,omitempty"`
	Rt_type     string `json:"rt_type,omitempty"`
}

//+kubebuilder:subresource:items

// RealTimeAppSpec defines the desired state of RealTimeApp
type RealTimeAppSpec struct {
	// Image is the image used for the realtimeapp
	Image           string               `json:"image,omitempty"`
	RealTimeThreads []RealTimeThreadSpec `json:"realtimethreads"`
	PodSpec         corev1.PodSpec       `json:"podspec,omitempty"`
}

// RealTimeAppStatus defines the observed state of RealTimeApp
type RealTimeAppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Deployment     string `json:"deployment,omitempty"`
	LastDeployment string `json:"last,omitempty"`
	State          string `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RealTimeApp is the Schema for the realtimeapps API
type RealTimeApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RealTimeAppSpec   `json:"spec,omitempty"`
	Status RealTimeAppStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RealTimeAppList contains a list of RealTimeApp
type RealTimeAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RealTimeApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RealTimeApp{}, &RealTimeAppList{})
}
