// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresources

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/andybalholm/brotli"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	forkedyaml "github.com/gardener/gardener/third_party/gopkg.in/yaml.v2"
)

// Registry stores objects and their serialized form. It allows to compute a map of all registered objects that can be
// used as part of a Secret's data which is referenced by a ManagedResource.
type Registry struct {
	scheme           *runtime.Scheme
	codec            runtime.Codec
	objects          []*object
	isYAMLSerializer bool
}

type object struct {
	obj           client.Object
	serialization []byte
}

// NewRegistry returns a new registry for resources. The given scheme, codec, and serializer must know all the resource
// types that will later be added to the registry.
func NewRegistry(scheme *runtime.Scheme, codec serializer.CodecFactory, serializer *jsonserializer.Serializer) *Registry {
	var groupVersions schema.GroupVersions
	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	// Use set to remove duplicates
	groupVersions = sets.New[schema.GroupVersion](groupVersions...).UnsortedList()

	// Sort groupVersions to ensure groupVersions.Identifier() is stable key
	// for the map in https://github.com/kubernetes/apimachinery/blob/v0.26.1/pkg/runtime/serializer/versioning/versioning.go#L94
	slices.SortStableFunc(groupVersions, func(a, b schema.GroupVersion) int {
		if a.Group == b.Group {
			return cmp.Compare(a.Version, b.Version)
		}
		return cmp.Compare(a.Group, b.Group)
	})

	// A workaround to incosistent/unstable ordering in yaml.v2 when encoding maps
	// Can be removed once k8s.io/apimachinery/pkg/runtime/serializer/json migrates to yaml.v3
	// or the issue is resolved upstream in yaml.v2
	// Please see https://github.com/go-yaml/yaml/pull/736
	serializerIdentifier := struct {
		YAML string `json:"yaml"`
	}{}

	utilruntime.Must(json.Unmarshal([]byte(serializer.Identifier()), &serializerIdentifier))

	return &Registry{
		scheme:           scheme,
		codec:            codec.CodecForVersions(serializer, serializer, groupVersions, groupVersions),
		isYAMLSerializer: serializerIdentifier.YAML == "true",
	}
}

// Add adds the given object to the registry. It computes a filename based on its type, namespace, and name. It serializes
// the object to YAML and stores both representations (object and serialization) in the registry.
func (r *Registry) Add(objs ...client.Object) error {
	for _, obj := range objs {
		if obj == nil || reflect.ValueOf(obj) == reflect.Zero(reflect.TypeOf(obj)) {
			continue
		}

		serializationYAML, err := runtime.Encode(r.codec, obj)
		if err != nil {
			return err
		}

		// We use a copy of the upstream package as a workaround
		// to incosistent/unstable ordering in yaml.v2 when encoding maps
		// Can be removed once k8s.io/apimachinery/pkg/runtime/serializer/json migrates to yaml.v3
		// or the issue is resolved upstream in yaml.v2
		// Please see https://github.com/go-yaml/yaml/pull/736
		if r.isYAMLSerializer {
			var anyObj interface{}
			if err := forkedyaml.Unmarshal(serializationYAML, &anyObj); err != nil {
				return err
			}

			serBytes, err := forkedyaml.Marshal(anyObj)
			if err != nil {
				return err
			}
			serializationYAML = serBytes
		}

		r.objects = append(r.objects, &object{
			obj:           obj,
			serialization: serializationYAML,
		})
	}

	return nil
}

// AddSerialized adds the provided serialized YAML for the registry. The provided filename will be used as key.
func (r *Registry) AddSerialized(serializationYAML []byte) {
	r.objects = append(r.objects, &object{serialization: serializationYAML})
}

// SerializedObjects returns a map whose keys are filenames and whose values are serialized objects.
func (r *Registry) SerializedObjects() map[string][]byte {
	var data []byte
	for _, object := range r.objects {
		data = append(data, append([]byte("---\n"), object.serialization...)...)
	}

	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		panic(err) // unreachable in hackathon
	}
	if err := w.Close(); err != nil {
		panic(err) // unreachable in hackathon
	}

	return map[string][]byte{resourcesv1alpha1.CompressedDataKey: buf.Bytes()}
}

// AddAllAndSerialize calls Add() for all the given objects before calling SerializedObjects().
func (r *Registry) AddAllAndSerialize(objects ...client.Object) (map[string][]byte, error) {
	if err := r.Add(objects...); err != nil {
		return nil, err
	}
	return r.SerializedObjects(), nil
}

// RegisteredObjects returns a map whose keys are filenames and whose values are objects.
func (r *Registry) RegisteredObjects() []client.Object {
	out := make([]client.Object, len(r.objects))
	for _, object := range r.objects {
		out = append(out, object.obj)
	}
	return out
}

// String returns the string representation of the registry.
func (r *Registry) String() string {
	out := make([]string, 0, len(r.objects))
	for _, object := range r.objects {
		out = append(out, fmt.Sprintf("---\n%s\n", object.serialization))
	}
	return strings.Join(out, "\n\n")
}
