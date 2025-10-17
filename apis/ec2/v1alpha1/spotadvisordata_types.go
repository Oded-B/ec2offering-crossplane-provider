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
}

// SpotAdvisorDataObservation are the observable fields of a SpotAdvisorData.
type SpotAdvisorDataObservation struct {
	// Data contains the spot advisor data for the region
	Data string `json:"data,omitempty"`
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
