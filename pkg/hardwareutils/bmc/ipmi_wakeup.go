package bmc

import (
	"fmt"
	"net/url"
)

const (
	ipmiWakeup = "ipmi-wakeup"
)

func init() {
	RegisterFactory("ipmi-wakeup", newIPMIWakeupAccessDetails, []string{})
	RegisterFactory("libvirt-wakeup", newIPMIWakeupAccessDetails, []string{})
}

func newIPMIWakeupAccessDetails(parsedURL *url.URL, disableCertificateVerification bool) (AccessDetails, error) {
	return &ipmiWakeupAccessDetails{
		bmcType:                        parsedURL.Scheme,
		portNum:                        parsedURL.Port(),
		hostname:                       parsedURL.Hostname(),
		privilegelevel:                 getPrivilegeLevel(parsedURL.RawQuery),
		disableCertificateVerification: disableCertificateVerification,
	}, nil
}

type ipmiWakeupAccessDetails struct {
	bmcType                        string
	portNum                        string
	hostname                       string
	privilegelevel                 string
	disableCertificateVerification bool
}

func (a *ipmiWakeupAccessDetails) Type() string {
	return a.bmcType
}

// NeedsMAC returns true when the host is going to need a separate
// port created rather than having it discovered.
func (a *ipmiWakeupAccessDetails) NeedsMAC() bool {
	// libvirt-based hosts used for dev and testing require a MAC
	// address, specified as part of the host, but we don't want the
	// provisioner to have to know the rules about which drivers
	// require what so we hide that detail inside this class and just
	// let the provisioner know that "some" drivers require a MAC and
	// it should ask.
	return a.bmcType == "libvirt"
}

func (a *ipmiWakeupAccessDetails) Driver() string {
	return "ipmi-wakeup"
}

func (a *ipmiWakeupAccessDetails) DisableCertificateVerification() bool {
	return a.disableCertificateVerification
}

// DriverInfo returns a data structure to pass as the DriverInfo
// parameter when creating a node in Ironic. The structure is
// pre-populated with the access information, and the caller is
// expected to add any other information that might be needed (such as
// the kernel and ramdisk locations).
func (a *ipmiWakeupAccessDetails) DriverInfo(bmcCreds Credentials) map[string]interface{} {
	result := map[string]interface{}{
		"ipmi_port":       a.portNum,
		"ipmi_username":   bmcCreds.Username,
		"ipmi_password":   bmcCreds.Password,
		"ipmi_address":    a.hostname,
		"ipmi_priv_level": a.privilegelevel,
	}

	if bmcCreds.SSHWakeup == SSHWakeupEnabled {
		result["wakeup_ssh_addr"] = bmcCreds.SSHAddress
		result["wakeup_ssh_user"] = bmcCreds.SSHUser
		result["wakeup_ssh_key"] = bmcCreds.SSHKey
	}

	if a.disableCertificateVerification {
		result["ipmi_verify_ca"] = false
	}
	if a.portNum == "" {
		result["ipmi_port"] = ipmiDefaultPort
	}
	return result
}

func (a *ipmiWakeupAccessDetails) BIOSInterface() string {
	return ""
}

func (a *ipmiWakeupAccessDetails) BootInterface() string {
	return ipmiWakeup
}

func (a *ipmiWakeupAccessDetails) FirmwareInterface() string {
	return ""
}

func (a *ipmiWakeupAccessDetails) ManagementInterface() string {
	return ""
}

func (a *ipmiWakeupAccessDetails) PowerInterface() string {
	return fake
}

func (a *ipmiWakeupAccessDetails) RAIDInterface() string {
	return noRaid
}

func (a *ipmiWakeupAccessDetails) VendorInterface() string {
	return ""
}

func (a *ipmiWakeupAccessDetails) SupportsSecureBoot() bool {
	return false
}

func (a *ipmiWakeupAccessDetails) SupportsISOPreprovisioningImage() bool {
	return false
}

func (a *ipmiWakeupAccessDetails) RequiresProvisioningNetwork() bool {
	return true
}

func (a *ipmiWakeupAccessDetails) BuildBIOSSettings(firmwareConfig *FirmwareConfig) (settings []map[string]string, err error) {
	if firmwareConfig != nil {
		return nil, fmt.Errorf("firmware settings for %s are not supported", a.Driver())
	}
	return nil, nil
}
