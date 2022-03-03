/*


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

// HostDataSpec defines the desired state of HostData
type HostDataSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The hardware discovered on the host during its inspection.
	HardwareDetails *HardwareDetails `json:"hardware,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=hostdata,scope=Namespaced,shortName=hd
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HostData"

// HostData is the Schema for the hostdata API
type HostData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HostDataSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// HostDataList contains a list of HostData
type HostDataList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostData `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostData{}, &HostDataList{})
}
