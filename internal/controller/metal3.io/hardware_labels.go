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
	"strconv"
	"strings"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
)

const acceleratorTypeGPU = "GPU"

// desiredAcceleratorLabels computes the set of GPU-related labels that
// should be present on a BareMetalHost, based on the accelerator devices
// found by hardware inspection.
func desiredAcceleratorLabels(details *metal3api.HardwareDetails) map[string]string {
	if details == nil {
		return nil
	}

	gpuCount := 0
	desired := map[string]string{}
	for _, acc := range details.Accelerators {
		if acc.Type != acceleratorTypeGPU {
			continue
		}
		gpuCount++
		if acc.VendorID != "" && acc.DeviceID != "" {
			desired[metal3api.GPUModelLabelPrefix+acc.VendorID+"-"+acc.DeviceID] = "true"
		}
	}

	if gpuCount == 0 {
		return desired
	}

	desired[metal3api.GPULabel] = "true"
	desired[metal3api.GPUCountLabel] = strconv.Itoa(gpuCount)
	return desired
}

// syncAcceleratorLabels updates host's labels to reflect the accelerator
// devices (e.g. GPUs) present in details, adding, updating or removing
// labels with the metal3api.GPULabel/metal3api.GPUModelLabelPrefix prefix
// as needed. It returns true if any label was added, changed or removed.
func syncAcceleratorLabels(host *metal3api.BareMetalHost, details *metal3api.HardwareDetails) bool {
	desired := desiredAcceleratorLabels(details)

	changed := false

	// Remove any stale GPU-related labels that are no longer applicable.
	for k := range host.Labels {
		if !isGPULabel(k) {
			continue
		}
		if _, ok := desired[k]; !ok {
			delete(host.Labels, k)
			changed = true
		}
	}

	if len(desired) == 0 {
		return changed
	}

	if host.Labels == nil {
		host.Labels = make(map[string]string, len(desired))
	}
	for k, v := range desired {
		if cur, ok := host.Labels[k]; !ok || cur != v {
			host.Labels[k] = v
			changed = true
		}
	}

	return changed
}

func isGPULabel(key string) bool {
	return key == metal3api.GPULabel || key == metal3api.GPUCountLabel || strings.HasPrefix(key, metal3api.GPUModelLabelPrefix)
}
