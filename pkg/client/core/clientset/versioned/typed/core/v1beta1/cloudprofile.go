// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	"context"

	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	scheme "github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// CloudProfilesGetter has a method to return a CloudProfileInterface.
// A group's client should implement this interface.
type CloudProfilesGetter interface {
	CloudProfiles() CloudProfileInterface
}

// CloudProfileInterface has methods to work with CloudProfile resources.
type CloudProfileInterface interface {
	Create(ctx context.Context, cloudProfile *v1beta1.CloudProfile, opts v1.CreateOptions) (*v1beta1.CloudProfile, error)
	Update(ctx context.Context, cloudProfile *v1beta1.CloudProfile, opts v1.UpdateOptions) (*v1beta1.CloudProfile, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, cloudProfile *v1beta1.CloudProfile, opts v1.UpdateOptions) (*v1beta1.CloudProfile, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.CloudProfile, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1beta1.CloudProfileList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.CloudProfile, err error)
	CloudProfileExpansion
}

// cloudProfiles implements CloudProfileInterface
type cloudProfiles struct {
	*gentype.ClientWithList[*v1beta1.CloudProfile, *v1beta1.CloudProfileList]
}

// newCloudProfiles returns a CloudProfiles
func newCloudProfiles(c *CoreV1beta1Client) *cloudProfiles {
	return &cloudProfiles{
		gentype.NewClientWithList[*v1beta1.CloudProfile, *v1beta1.CloudProfileList](
			"cloudprofiles",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *v1beta1.CloudProfile { return &v1beta1.CloudProfile{} },
			func() *v1beta1.CloudProfileList { return &v1beta1.CloudProfileList{} }),
	}
}
