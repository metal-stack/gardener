package lease_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = FDescribe("Reconcile", func() {

	Describe("Lease controller tests", func() {
		var node *corev1.Node

		BeforeEach(func() {
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   nodeName,
					Labels: map[string]string{testID: testRunID},
				},
			}

			By("Create Node")
			Expect(testClient.Create(ctx, node)).To(Succeed())
			DeferCleanup(func() {
				By("Delete Node")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
				By("Delete Lease")
				lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}}
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, lease))).To(Succeed())
			})
		})

		It("Reconcile should create lease", func() {
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, &coordinationv1.Lease{})
			}).Should(Succeed())
		})

		It("Reconcile should update lease time", func() {
			lease := &coordinationv1.Lease{}
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)
			}).Should(Succeed())
			oldRenewTime := lease.Spec.RenewTime

			metav1.SetMetaDataAnnotation(&node.ObjectMeta, "foo", "bar")
			Expect(testClient.Update(ctx, node)).To(Succeed())

			Eventually(func() bool {
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)).To(Succeed())
				return lease.Spec.RenewTime.After(oldRenewTime.Time)
			}).Should(BeTrue())
		})

		It("Reconcile should not update lease time if no node is present", func() {
			lease := &coordinationv1.Lease{}
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)
			}).Should(Succeed())

			oldRenewTime := lease.Spec.RenewTime

			Expect(testClient.Delete(ctx, node)).To(Succeed())
			Eventually(func() error {
				return testClient.Get(ctx, types.NamespacedName{Name: nodeName}, node)
			}).Should(BeNotFoundError())

			Consistently(func() bool {
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNamespace.Name, Name: "gardener-node-agent-" + nodeName}, lease)).To(Succeed())
				return lease.Spec.RenewTime.Equal(oldRenewTime)
			}).Should(BeTrue())
		})
	})
})
