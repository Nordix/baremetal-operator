package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
)

var _ = Describe("Provisioning", func() {
	var (
		specName       = "provisioning-ops"
		secretName     = "bmc-credentials"
		namespace      *corev1.Namespace
		cancelWatches  context.CancelFunc
		bmcUser        string
		bmcPassword    string
		bmcAddress     string
		bootMacAddress string
	)

	BeforeEach(func() {
		bmcUser = e2eConfig.GetVariable("BMC_USER")
		bmcPassword = e2eConfig.GetVariable("BMC_PASSWORD")
		bmcAddress = e2eConfig.GetVariable("BMC_ADDRESS")
		bootMacAddress = e2eConfig.GetVariable("BOOT_MAC_ADDRESS")

		namespace, cancelWatches = framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
			Creator:   clusterProxy.GetClient(),
			ClientSet: clusterProxy.GetClientSet(),
			Name:      fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
			LogFolder: artifactFolder,
		})
	})

	It("should provision and then deprovision a BMH", func() {
		By("Creating a secret with BMH credentials")
		client := clusterProxy.GetClient()
		CreateBMHCredentialsSecret(ctx, client, namespace.Name, secretName, bmcUser, bmcPassword)

		By("Creating a BMH with inspection disabled and hardware details added")
		bmh := metal3api.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      specName,
				Namespace: namespace.Name,
				Annotations: map[string]string{
					metal3api.InspectAnnotationPrefix:   "disabled",
					metal3api.HardwareDetailsAnnotation: hardwareDetails,
				},
			},
			Spec: metal3api.BareMetalHostSpec{
				Online: true,
				BMC: metal3api.BMCDetails{
					Address:         bmcAddress,
					CredentialsName: "bmc-credentials",
				},
				BootMode:       metal3api.Legacy,
				BootMACAddress: bootMacAddress,
			},
		}
		err := clusterProxy.GetClient().Create(ctx, &bmh)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for the BMH to be in registering state")
		err = WaitForBmhState(
			ctx,
			clusterProxy.GetClient(),
			bmh.Namespace,
			bmh.Name,
			metal3api.StateRegistering,
			[]metal3api.ProvisioningState{}, // No undesired states
			e2eConfig.GetIntervals(specName, "wait-registering"),
		)
		Expect(err).NotTo(HaveOccurred())
		
		By("Waiting for the BMH to become available")
		err = WaitForBmhState(
			ctx,
			clusterProxy.GetClient(),
			bmh.Namespace,
			bmh.Name,
			metal3api.StateAvailable,
			[]metal3api.ProvisioningState{}, // No undesired states
			e2eConfig.GetIntervals(specName, "wait-available"),
		)
		Expect(err).NotTo(HaveOccurred())

		By("Patching the BMH to test provisioning")
		helper, err := patch.NewHelper(&bmh, clusterProxy.GetClient())
		Expect(err).NotTo(HaveOccurred())
		bmh.Spec.Image = &metal3api.Image{
			URL:      e2eConfig.GetVariable("IMAGE_URL"),
			Checksum: e2eConfig.GetVariable("IMAGE_CHECKSUM"),
		}
		bmh.Spec.RootDeviceHints = &metal3api.RootDeviceHints{
			DeviceName: "/dev/vda",
		}
		Expect(helper.Patch(ctx, &bmh)).To(Succeed())

		By("Waiting for the BMH to be in provisioning state")
		err = WaitForBmhState(
			ctx,
			clusterProxy.GetClient(),
			bmh.Namespace,
			bmh.Name,
			metal3api.StateProvisioning,
			[]metal3api.ProvisioningState{}, // No undesired states
			e2eConfig.GetIntervals(specName, "wait-provisioning"),
		)
		Expect(err).NotTo(HaveOccurred())
		
		By("Waiting for the BMH to become provisioned")
		err = WaitForBmhState(
			ctx,
			clusterProxy.GetClient(),
			bmh.Namespace,
			bmh.Name,
			metal3api.StateProvisioned,
			[]metal3api.ProvisioningState{}, // No undesired states
			e2eConfig.GetIntervals(specName, "wait-provisioned"),
		)

		By("Triggering the deprovisioning of the BMH")
		helper, err = patch.NewHelper(&bmh, clusterProxy.GetClient())
		Expect(err).NotTo(HaveOccurred())
		bmh.Spec.Image = nil
		Expect(helper.Patch(ctx, &bmh)).To(Succeed())

		By("Waiting for the BMH to be in deprovisioning state")
		err = WaitForBmhState(
			ctx,
			clusterProxy.GetClient(),
			bmh.Namespace,
			bmh.Name,
			metal3api.StateDeprovisioning,
			[]metal3api.ProvisioningState{}, // No undesired states
			e2eConfig.GetIntervals(specName, "wait-deprovisioning"),
		)
		Expect(err).NotTo(HaveOccurred())
		
		By("Waiting for the BMH to become available again")
		err = WaitForBmhState(
			ctx,
			clusterProxy.GetClient(),
			bmh.Namespace,
			bmh.Name,
			metal3api.StateAvailable,
			[]metal3api.ProvisioningState{}, // No undesired states
			e2eConfig.GetIntervals(specName, "wait-available"),
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cleanup(ctx, clusterProxy, namespace, cancelWatches, e2eConfig.GetIntervals("default", "wait-namespace-deleted")...)
	})
})
