package token_test

import (
	"context"
	"path"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/kubeletupgrade"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardencontroller "github.com/gardener/gardener/pkg/operator/controller/garden"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/spf13/afero"
)

var _ = Describe("Nodeagent token controller tests", func() {
	var (
		loadBalancerServiceAnnotations = map[string]string{"foo": "bar"}
		garden                         *operatorv1alpha1.Garden
		testRunID                      string
		testNamespace                  *corev1.Namespace
		testFs                         afero.Fs
		nodeAgentToken                 string
		kubeletPath                    string
		controllerTriggerChannel       chan event.GenericEvent
		nodeAgentConfig                *nodeagentv1alpha1.NodeAgentConfiguration
		fakeExtractor                  *registry.FakeRegistryExtractor
		fakeDbus                       *dbus.FakeDbus
	)
	const (
		contractHyperkubeKubeletName = "kubelet"
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, true))
		DeferCleanup(test.WithVars(
			&etcd.DefaultInterval, 100*time.Millisecond,
			&etcd.DefaultTimeout, 500*time.Millisecond,
			&kubeapiserverexposure.DefaultInterval, 100*time.Millisecond,
			&kubeapiserverexposure.DefaultTimeout, 500*time.Millisecond,
			&kubeapiserver.IntervalWaitForDeployment, 100*time.Millisecond,
			&kubeapiserver.TimeoutWaitForDeployment, 500*time.Millisecond,
			&kubeapiserver.Until, untilInTest,
			&kubecontrollermanager.IntervalWaitForDeployment, 100*time.Millisecond,
			&kubecontrollermanager.TimeoutWaitForDeployment, 500*time.Millisecond,
			&kubecontrollermanager.Until, untilInTest,
			&resourcemanager.SkipWebhookDeployment, true,
			&resourcemanager.IntervalWaitForDeployment, 100*time.Millisecond,
			&resourcemanager.TimeoutWaitForDeployment, 500*time.Millisecond,
			&resourcemanager.Until, untilInTest,
			&shared.IntervalWaitForGardenerResourceManagerBootstrapping, 500*time.Millisecond,
		))

		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "garden-",
			},
		}
		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)
		testRunID = testNamespace.Name

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Setup manager")
		mapper, err := apiutil.NewDynamicRESTMapper(restConfig)
		Expect(err).NotTo(HaveOccurred())

		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:             operatorclient.RuntimeScheme,
			MetricsBindAddress: "0",
			NewCache: cache.BuilderWithOptions(cache.Options{
				Mapper: mapper,
				SelectorsByObject: map[client.Object]cache.ObjectSelector{
					&operatorv1alpha1.Garden{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					},
				},
			}),
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		By("Register controller")
		testFs = afero.NewMemMapFs()
		nodeAgentConfig = &nodeagentv1alpha1.NodeAgentConfiguration{
			TokenSecretName: v1alpha1.NodeAgentTokenSecretName,
			HyperkubeImage:  images.ImageNameHyperkube,
		}
		configBytes, err := yaml.Marshal(nodeAgentConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentConfigPath, configBytes, 0644)).To(Succeed())

		// TODO: remove?
		nodeAgentToken = "original-node-agent-token"
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentTokenFilePath, []byte(nodeAgentToken), 0644)).To(Succeed())

		kubeletPath = "/opt/bin/kubelet"

		controllerTriggerChannel = make(chan event.GenericEvent)
		fakeExtractor = &registry.FakeRegistryExtractor{}
		fakeDbus = &dbus.FakeDbus{}

		kubeletReconciler := &kubeletupgrade.Reconciler{
			Client:           mgr.GetClient(),
			Fs:               testFs,
			Config:           nodeAgentConfig,
			TriggerChannel:   controllerTriggerChannel,
			Extractor:        fakeExtractor,
			Dbus:             fakeDbus,
			TargetBinaryPath: kubeletPath,
			// TODO: add fields?
		}
		Expect((kubeletReconciler.AddToManager(mgr))).To(Succeed())

		By("Start manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("Stop manager")
			mgrCancel()
		})

		DeferCleanup(test.WithVar(&gardencontroller.NewClientFromSecretObject, func(secret *corev1.Secret, fns ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
			Expect(secret.Name).To(Equal("gardener-internal"))
			Expect(secret.Namespace).To(Equal(testNamespace.Name))
			return kubernetesfake.NewClientSetBuilder().WithClient(testClient).Build(), nil
		}))

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "garden-" + testRunID,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     "10.1.0.0/16",
						Services: "10.2.0.0/16",
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
					Settings: &operatorv1alpha1.Settings{
						LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{
							Annotations: loadBalancerServiceAnnotations,
						},
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: pointer.Bool(true),
						},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domain: "virtual-garden.local.gardener.cloud",
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.26.3",
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Networking: operatorv1alpha1.Networking{
						Services: "100.64.0.0/13",
					},
				},
			},
		}

		By("Create Garden")
		Expect(testClient.Create(ctx, garden)).To(Succeed())
		log.Info("Created Garden for test", "garden", garden.Name)

		DeferCleanup(func() {
			By("Delete Garden")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())

			By("Forcefully remove finalizers")
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, garden))).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
			}).Should(BeNotFoundError())
		})
	})

	It("should update and restart kubelet when channel triggered", func() {
		controllerTriggerChannel <- event.GenericEvent{}

		Eventually(func(g Gomega) error {
			g.Expect(fakeExtractor.Extractions).To(HaveLen(1))
			actual := fakeExtractor.Extractions[0]
			g.Expect(actual.Image).To(Equal(nodeAgentConfig.HyperkubeImage))
			g.Expect(actual.PathSuffix).To(Equal(contractHyperkubeKubeletName))
			g.Expect(actual.Dest).To(Equal(kubeletPath))

			g.Expect(fakeDbus.Actions).To(HaveLen(1))
			g.Expect(fakeDbus.Actions[0].Action).To(Equal(dbus.FakeRestart))
			g.Expect(fakeDbus.Actions[0].UnitNames).To(Equal([]string{kubelet.UnitName}))
			return nil
		}).Should(Succeed())
	})

	It("should skip update and not restart kubelet when channel not triggered", func() {
		hyperkubeImageDownloadedPath := path.Join(nodeagentv1alpha1.NodeAgentBaseDir, "hyperkube-downloaded")

		Expect(afero.WriteFile(testFs, hyperkubeImageDownloadedPath, []byte(nodeAgentConfig.HyperkubeImage), 0644)).Should(Succeed())

		controllerTriggerChannel <- event.GenericEvent{}

		Consistently(func(g Gomega) error {
			g.Expect(fakeDbus.Actions).To(HaveLen(0))
			return nil
		}).Should(Succeed())
	})
})

func untilInTest(_ context.Context, _ time.Duration, _ retry.Func) error {
	return nil
}
