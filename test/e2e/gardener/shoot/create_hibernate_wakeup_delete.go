// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/node"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot

		It("Create, Hibernate, Wake up and Delete Shoot", Offset(1), func() {
			By("Create Shoot")
			ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			if !v1beta1helper.IsWorkerless(f.Shoot) {
				By("Verify Bootstrapping of Nodes with node-critical components")
				// We verify the node readiness feature in this specific e2e test because it uses a single-node shoot cluster.
				// The default shoot e2e test deals with multiple nodes, deleting all of them and waiting for them to be recreated
				// might increase the test duration undesirably.
				ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
				defer cancel()
				node.VerifyNodeCriticalComponentsBootstrapping(ctx, f.ShootFramework)
			}

			By("Hibernate Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.HibernateShoot(ctx, f.Shoot)).To(Succeed())

			By("Wake up Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.WakeUpShoot(ctx, f.Shoot)).To(Succeed())

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", Label("basic"), func() {
		test(e2e.DefaultShoot("e2e-wake-up"))
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-wake-up"))
	})
})
