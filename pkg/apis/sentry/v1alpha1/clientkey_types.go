package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClientKeySpec defines the desired state of ClientKey
type ClientKeySpec struct {
	Name        string `json:"name"`
	ProjectSlug string `json:"projectSlug"`
}

// ClientKeyStatus defines the observed state of ClientKey
type ClientKeyStatus struct {
	ID string `json:"id"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClientKey is the Schema for the clientkeys API
// +k8s:openapi-gen=true
type ClientKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientKeySpec   `json:"spec,omitempty"`
	Status ClientKeyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClientKeyList contains a list of ClientKey
type ClientKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClientKey{}, &ClientKeyList{})
}
