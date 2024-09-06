package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BookStore is a resource.
type BookStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   BookStoreSpec   `json:"spec"`
	Status BookStoreStatus `json:"status,omitempty"`
}
type BookStoreStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BookStoreList is a collection of BookStore resources.
type BookStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []BookStore `json:"items"`
}

// BookStoreSpec is the spec of a BookStore resource.
type BookStoreSpec struct {
	Name      string        `json:"name"`
	Replicas  *int32        `json:"replicas"`
	Container ContainerSpec `json:"container,container"`
}

// ContainerSpec contains specs of container
type ContainerSpec struct {
	Image string `json:"image,omitempty"`
	Port  int32  `json:"port,omitempty"`
}
