// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Network", func() {
	var (
		ctrl     *gomock.Controller
		network  *mockcomponent.MockDeployMigrateWaiter
		botanist *Botanist

		ctx        = context.TODO()
		fakeErr    = errors.New("fake")
		shootState = &gardencorev1beta1.ShootState{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		network = mockcomponent.NewMockDeployMigrateWaiter(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						Network: network,
					},
				},
			},
		}}
		botanist.Shoot.SetShootState(shootState)
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployNetwork", func() {
		Context("deploy", func() {
			It("should deploy successfully", func() {
				network.EXPECT().Deploy(ctx)
				Expect(botanist.DeployNetwork(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				network.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployNetwork(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
			})

			It("should restore successfully", func() {
				network.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployNetwork(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				network.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployNetwork(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
