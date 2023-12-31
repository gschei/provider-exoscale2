/*
Copyright 2022 The Crossplane Authors.

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

type ExoscaleSKSParameters struct {

	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Zone string `json:"zone"`
	Cni  string `json:"cni"`
	// +kubebuilder:validation:Required
	ServiceLevel string `json:"serviceLevel"`
	// +kubebuilder:validation:Required
	NodepoolName string `json:"nodepoolName"`
	// +kubebuilder:validation:Required
	NodepoolSize           int    `json:"nodepoolSize"`
	NodepoolSecurityGroup  string `json:"nodepoolSecurityGroup,omitempty"`
	NodepoolInstanceType   string `json:"nodepoolInstanceType,omitempty"`
	NodepoolDiskSize       int    `json:"nodepoolDiskSize,omitempty"`
	NodepoolPrivateNetwork string `json:"nodepoolPrivateNetwork,omitempty"`
}

type ExoscaleSKSObservation struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// A ExoscaleSKSSpec defines the desired state of a ExoscaleSKS.
type ExoscaleSKSSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ExoscaleSKSParameters `json:"forProvider"`
}

// A ExoscaleSKSStatus represents the observed state of a ExoscaleSKS.
type ExoscaleSKSStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ExoscaleSKSObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A ExoscaleSKS is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,exoscale2}
type ExoscaleSKS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExoscaleSKSSpec   `json:"spec"`
	Status ExoscaleSKSStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExoscaleSKSList contains a list of ExoscaleSKS
type ExoscaleSKSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExoscaleSKS `json:"items"`
}

// ExoscaleSKS type metadata.
var (
	ExoscaleSKSKind             = reflect.TypeOf(ExoscaleSKS{}).Name()
	ExoscaleSKSGroupKind        = schema.GroupKind{Group: Group, Kind: ExoscaleSKSKind}.String()
	ExoscaleSKSKindAPIVersion   = ExoscaleSKSKind + "." + SchemeGroupVersion.String()
	ExoscaleSKSGroupVersionKind = SchemeGroupVersion.WithKind(ExoscaleSKSKind)
)

func init() {
	SchemeBuilder.Register(&ExoscaleSKS{}, &ExoscaleSKSList{})
}
