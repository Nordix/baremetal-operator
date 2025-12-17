package bmc

import (
	"net/url"
)

const (
	redfishWakeup = "redfish-wakeup"
	fake          = "fake"
)

func init() {
	schemes := []string{"http", "https"}
	RegisterFactory("redfish-wakeup", newRedfishWakeupAccessDetails, schemes)
	RegisterFactory("ilo5-wakeup", newRedfishWakeupAccessDetails, schemes)
	RegisterFactory("idrac-wakeup", newRedfishWakeupAccessDetails, schemes)
}

func newRedfishWakeupAccessDetails(parsedURL *url.URL, disableCertificateVerification bool) (AccessDetails, error) {
	return &redfishWakeupAccessDetails{
		redfishAccessDetails{
			bmcType:                        parsedURL.Scheme,
			host:                           parsedURL.Host,
			path:                           parsedURL.Path,
			disableCertificateVerification: disableCertificateVerification,
		},
	}, nil
}

type redfishWakeupAccessDetails struct {
	redfishAccessDetails
}

func (a *redfishWakeupAccessDetails) Type() string {
	return a.bmcType
}

// NeedsMAC returns false for virtual media drivers since they can boot
// from virtual media without requiring a pre-configured boot MAC address.
// The MAC address can be populated after hardware inspection completes.
func (a *redfishWakeupAccessDetails) NeedsMAC() bool {
	return false
}

func (a *redfishWakeupAccessDetails) Driver() string {
	return "redfish-wakeup"
}

func (a *redfishWakeupAccessDetails) DisableCertificateVerification() bool {
	return a.disableCertificateVerification
}

// DriverInfo returns a data structure to pass as the DriverInfo
// parameter when creating a node in Ironic. The structure is
// pre-populated with the access information, and the caller is
// expected to add any other information that might be needed (such as
// the kernel and ramdisk locations).
// Extended with the ssh-wakeup related credentials.
func (a *redfishWakeupAccessDetails) DriverInfo(bmcCreds Credentials) map[string]interface{} {
	driverInfo := a.redfishAccessDetails.DriverInfo(bmcCreds)
	if bmcCreds.SSHWakeup == SSHWakeupEnabled {
		driverInfo["wakeup_ssh_addr"] = bmcCreds.SSHAddress
		driverInfo["wakeup_ssh_user"] = bmcCreds.SSHUser
		driverInfo["wakeup_ssh_key"] = bmcCreds.SSHKey
	}
	return driverInfo
}

func (a *redfishWakeupAccessDetails) BIOSInterface() string {
	return ""
}

func (a *redfishWakeupAccessDetails) BootInterface() string {
	return redfishWakeup
}

func (a *redfishWakeupAccessDetails) FirmwareInterface() string {
	return redfish
}

func (a *redfishWakeupAccessDetails) ManagementInterface() string {
	return ""
}

func (a *redfishWakeupAccessDetails) PowerInterface() string {
	return fake
}

func (a *redfishWakeupAccessDetails) RAIDInterface() string {
	return redfish
}

func (a *redfishWakeupAccessDetails) VendorInterface() string {
	return ""
}

// Well as much as kexec and/or systemd-soft reboot supports secure boot.
func (a *redfishWakeupAccessDetails) SupportsSecureBoot() bool {
	return true
}

// Well yes and no, it doesn't care if Ironic has an ISO preprov image
// configured on per node or on global level because the driver will wake up
// an IPA stored on the target machine in initrd+kernel format.
func (a *redfishWakeupAccessDetails) SupportsISOPreprovisioningImage() bool {
	return true
}

func (a *redfishWakeupAccessDetails) RequiresProvisioningNetwork() bool {
	return false
}

func (a *redfishWakeupAccessDetails) BuildBIOSSettings(firmwareConfig *FirmwareConfig) (settings []map[string]string, err error) {
	return a.redfishAccessDetails.BuildBIOSSettings(firmwareConfig)
}
