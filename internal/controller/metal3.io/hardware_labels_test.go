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

package controllers

import (
	"testing"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncAcceleratorLabelsNoGPUs(t *testing.T) {
	host := &metal3api.BareMetalHost{}
	changed := syncAcceleratorLabels(host, &metal3api.HardwareDetails{})
	assert.False(t, changed)
	assert.Empty(t, host.Labels)
}

func TestSyncAcceleratorLabelsNilDetails(t *testing.T) {
	host := &metal3api.BareMetalHost{}
	changed := syncAcceleratorLabels(host, nil)
	assert.False(t, changed)
	assert.Empty(t, host.Labels)
}

func TestSyncAcceleratorLabelsAddsLabels(t *testing.T) {
	host := &metal3api.BareMetalHost{}
	details := &metal3api.HardwareDetails{
		Accelerators: []metal3api.Accelerator{
			{VendorID: "10de", DeviceID: "1eb8", Type: "GPU", DeviceInfo: "NVIDIA Tesla T4"},
			{VendorID: "10de", DeviceID: "1eb8", Type: "GPU", DeviceInfo: "NVIDIA Tesla T4"},
			{VendorID: "8086", DeviceID: "1572", Type: "NIC"}, // not a GPU, ignored
		},
	}

	changed := syncAcceleratorLabels(host, details)
	assert.True(t, changed)
	assert.Equal(t, "true", host.Labels[metal3api.GPULabel])
	assert.Equal(t, "2", host.Labels[metal3api.GPUCountLabel])
	assert.Equal(t, "true", host.Labels["hardware.metal3.io/gpu-10de-1eb8"])
	_, hasNICLabel := host.Labels["hardware.metal3.io/gpu-8086-1572"]
	assert.False(t, hasNICLabel)

	// Calling again with the same data should be a no-op.
	changed = syncAcceleratorLabels(host, details)
	assert.False(t, changed)
}

func TestSyncAcceleratorLabelsRemovesStaleLabels(t *testing.T) {
	host := &metal3api.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				metal3api.GPULabel:                 "true",
				metal3api.GPUCountLabel:            "1",
				"hardware.metal3.io/gpu-10de-1eb8": "true",
				"unrelated-label":                  "keep-me",
			},
		},
	}

	// Re-inspection now reports no GPUs at all.
	changed := syncAcceleratorLabels(host, &metal3api.HardwareDetails{})
	assert.True(t, changed)
	assert.NotContains(t, host.Labels, metal3api.GPULabel)
	assert.NotContains(t, host.Labels, metal3api.GPUCountLabel)
	assert.NotContains(t, host.Labels, "hardware.metal3.io/gpu-10de-1eb8")
	assert.Equal(t, "keep-me", host.Labels["unrelated-label"])
}

func TestSyncAcceleratorLabelsModelChange(t *testing.T) {
	host := &metal3api.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				metal3api.GPULabel:                 "true",
				metal3api.GPUCountLabel:            "1",
				"hardware.metal3.io/gpu-10de-1eb8": "true",
			},
		},
	}

	// Host is now inspected with a different GPU model.
	details := &metal3api.HardwareDetails{
		Accelerators: []metal3api.Accelerator{
			{VendorID: "10de", DeviceID: "2236", Type: "GPU", DeviceInfo: "NVIDIA A10"},
		},
	}
	changed := syncAcceleratorLabels(host, details)
	assert.True(t, changed)
	assert.Equal(t, "true", host.Labels[metal3api.GPULabel])
	assert.Equal(t, "1", host.Labels[metal3api.GPUCountLabel])
	assert.NotContains(t, host.Labels, "hardware.metal3.io/gpu-10de-1eb8")
	assert.Equal(t, "true", host.Labels["hardware.metal3.io/gpu-10de-2236"])
}
