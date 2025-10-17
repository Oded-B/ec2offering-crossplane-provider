/*
Copyright 2025 The Crossplane Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// SpotAdvisorDataParameters are the configurable fields of a SpotAdvisorData.
type SpotAdvisorDataParameters struct {
	// AWSRegion specifies the AWS region to query for spot advisor data
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=^[a-z0-9-]+$
	AWSRegion string `json:"awsRegion"`

	// OS specifies the operating system to filter for (Linux, Windows, etc.)
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=Linux
	// +kubebuilder:validation:Enum=Linux;Windows
	OS *string `json:"os,omitempty"`
}

// SpotAdvisorDataObservation are the observable fields of a SpotAdvisorData.
type SpotAdvisorDataObservation struct {
	// InstanceTypes contains the instance types data from spot advisor
	InstanceTypes map[string]InstanceTypeData `json:"instanceTypes,omitempty"`
	// SpotAdvisor contains the spot advisor data filtered by region
	SpotAdvisor map[string]map[string]RegionData `json:"spotAdvisor,omitempty"`
	// GlobalRate contains the global rate information
	GlobalRate string `json:"globalRate,omitempty"`
	// Ranges contains the ranges data from spot advisor
	Ranges []RangeData `json:"ranges,omitempty"`
}

// InstanceTypeData represents instance type information
type InstanceTypeData struct {
	EMR   bool   `json:"emr,omitempty"`
	Cores int    `json:"cores,omitempty"`
	RAMGB string `json:"ram_gb,omitempty"`
}

// RegionData represents spot advisor data for a specific region
type RegionData struct {
	S int `json:"s,omitempty"` // Spot interruption frequency
	R int `json:"r,omitempty"` // Spot interruption rate
}

// RangeData represents range information from spot advisor
type RangeData struct {
	Index int    `json:"index,omitempty"` // Index of the range
	Label string `json:"label,omitempty"` // Label for the range (e.g., "<5%", "5-10%")
	Dots  int    `json:"dots,omitempty"`  // Number of dots
	Max   int    `json:"max,omitempty"`   // Maximum value
}

// A SpotAdvisorDataSpec defines the desired state of a SpotAdvisorData.
type SpotAdvisorDataSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       SpotAdvisorDataParameters `json:"forProvider"`
}

// A SpotAdvisorDataStatus represents the observed state of a SpotAdvisorData.
type SpotAdvisorDataStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SpotAdvisorDataObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A SpotAdvisorData is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,ec2offering}
type SpotAdvisorData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SpotAdvisorDataSpec   `json:"spec"`
	Status SpotAdvisorDataStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpotAdvisorDataList contains a list of SpotAdvisorData
type SpotAdvisorDataList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpotAdvisorData `json:"items"`
}

// SpotAdvisorData type metadata.
var (
	SpotAdvisorDataKind             = reflect.TypeOf(SpotAdvisorData{}).Name()
	SpotAdvisorDataGroupKind        = schema.GroupKind{Group: Group, Kind: SpotAdvisorDataKind}.String()
	SpotAdvisorDataKindAPIVersion   = SpotAdvisorDataKind + "." + SchemeGroupVersion.String()
	SpotAdvisorDataGroupVersionKind = SchemeGroupVersion.WithKind(SpotAdvisorDataKind)
)

func init() {
	SchemeBuilder.Register(&SpotAdvisorData{}, &SpotAdvisorDataList{})
}
