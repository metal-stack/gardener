// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: github.com/gardener/gardener/pkg/apis/core/v1/generated.proto

package v1

import (
	fmt "fmt"

	io "io"

	proto "github.com/gogo/protobuf/proto"
	v11 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	math "math"
	math_bits "math/bits"
	reflect "reflect"
	strings "strings"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

func (m *ControllerDeployment) Reset()      { *m = ControllerDeployment{} }
func (*ControllerDeployment) ProtoMessage() {}
func (*ControllerDeployment) Descriptor() ([]byte, []int) {
	return fileDescriptor_9b216bec51effd5c, []int{0}
}
func (m *ControllerDeployment) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ControllerDeployment) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	b = b[:cap(b)]
	n, err := m.MarshalToSizedBuffer(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (m *ControllerDeployment) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ControllerDeployment.Merge(m, src)
}
func (m *ControllerDeployment) XXX_Size() int {
	return m.Size()
}
func (m *ControllerDeployment) XXX_DiscardUnknown() {
	xxx_messageInfo_ControllerDeployment.DiscardUnknown(m)
}

var xxx_messageInfo_ControllerDeployment proto.InternalMessageInfo

func (m *ControllerDeploymentList) Reset()      { *m = ControllerDeploymentList{} }
func (*ControllerDeploymentList) ProtoMessage() {}
func (*ControllerDeploymentList) Descriptor() ([]byte, []int) {
	return fileDescriptor_9b216bec51effd5c, []int{1}
}
func (m *ControllerDeploymentList) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ControllerDeploymentList) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	b = b[:cap(b)]
	n, err := m.MarshalToSizedBuffer(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (m *ControllerDeploymentList) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ControllerDeploymentList.Merge(m, src)
}
func (m *ControllerDeploymentList) XXX_Size() int {
	return m.Size()
}
func (m *ControllerDeploymentList) XXX_DiscardUnknown() {
	xxx_messageInfo_ControllerDeploymentList.DiscardUnknown(m)
}

var xxx_messageInfo_ControllerDeploymentList proto.InternalMessageInfo

func (m *HelmControllerDeployment) Reset()      { *m = HelmControllerDeployment{} }
func (*HelmControllerDeployment) ProtoMessage() {}
func (*HelmControllerDeployment) Descriptor() ([]byte, []int) {
	return fileDescriptor_9b216bec51effd5c, []int{2}
}
func (m *HelmControllerDeployment) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *HelmControllerDeployment) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	b = b[:cap(b)]
	n, err := m.MarshalToSizedBuffer(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (m *HelmControllerDeployment) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HelmControllerDeployment.Merge(m, src)
}
func (m *HelmControllerDeployment) XXX_Size() int {
	return m.Size()
}
func (m *HelmControllerDeployment) XXX_DiscardUnknown() {
	xxx_messageInfo_HelmControllerDeployment.DiscardUnknown(m)
}

var xxx_messageInfo_HelmControllerDeployment proto.InternalMessageInfo

func (m *OCIRepository) Reset()      { *m = OCIRepository{} }
func (*OCIRepository) ProtoMessage() {}
func (*OCIRepository) Descriptor() ([]byte, []int) {
	return fileDescriptor_9b216bec51effd5c, []int{3}
}
func (m *OCIRepository) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *OCIRepository) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	b = b[:cap(b)]
	n, err := m.MarshalToSizedBuffer(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (m *OCIRepository) XXX_Merge(src proto.Message) {
	xxx_messageInfo_OCIRepository.Merge(m, src)
}
func (m *OCIRepository) XXX_Size() int {
	return m.Size()
}
func (m *OCIRepository) XXX_DiscardUnknown() {
	xxx_messageInfo_OCIRepository.DiscardUnknown(m)
}

var xxx_messageInfo_OCIRepository proto.InternalMessageInfo

func init() {
	proto.RegisterType((*ControllerDeployment)(nil), "github.com.gardener.gardener.pkg.apis.core.v1.ControllerDeployment")
	proto.RegisterType((*ControllerDeploymentList)(nil), "github.com.gardener.gardener.pkg.apis.core.v1.ControllerDeploymentList")
	proto.RegisterType((*HelmControllerDeployment)(nil), "github.com.gardener.gardener.pkg.apis.core.v1.HelmControllerDeployment")
	proto.RegisterType((*OCIRepository)(nil), "github.com.gardener.gardener.pkg.apis.core.v1.OCIRepository")
}

func init() {
	proto.RegisterFile("github.com/gardener/gardener/pkg/apis/core/v1/generated.proto", fileDescriptor_9b216bec51effd5c)
}

var fileDescriptor_9b216bec51effd5c = []byte{
	// 590 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x54, 0xc1, 0x6e, 0xd3, 0x4c,
	0x18, 0x8c, 0x9b, 0x36, 0x4a, 0xb7, 0xcd, 0xaf, 0x1f, 0x8b, 0x83, 0x15, 0x09, 0xa7, 0xca, 0x01,
	0xe5, 0x92, 0x35, 0x89, 0x10, 0xe2, 0x00, 0x1c, 0x9c, 0x4a, 0xb4, 0xa8, 0x10, 0x69, 0x0b, 0x1c,
	0x10, 0x12, 0x6c, 0x9c, 0xc5, 0x36, 0xb1, 0xbd, 0xd6, 0x7a, 0x1d, 0xc8, 0x8d, 0x47, 0xe0, 0x0d,
	0x38, 0xf3, 0x26, 0x39, 0xf6, 0xd8, 0x53, 0x20, 0xe6, 0x45, 0xd0, 0x6e, 0xdc, 0xac, 0x4d, 0x53,
	0x41, 0x6e, 0xeb, 0xd9, 0x99, 0xf9, 0x66, 0x3e, 0x3b, 0x01, 0x8f, 0x5d, 0x9f, 0x7b, 0xe9, 0x08,
	0x3a, 0x34, 0xb4, 0x5c, 0xcc, 0xc6, 0x24, 0x22, 0x4c, 0x1d, 0xe2, 0x89, 0x6b, 0xe1, 0xd8, 0x4f,
	0x2c, 0x87, 0x32, 0x62, 0x4d, 0x7b, 0x96, 0x2b, 0x60, 0xcc, 0xc9, 0x18, 0xc6, 0x8c, 0x72, 0xaa,
	0x77, 0x95, 0x1c, 0x5e, 0xa9, 0xd4, 0x21, 0x9e, 0xb8, 0x50, 0xc8, 0xa1, 0x90, 0xc3, 0x69, 0xaf,
	0xd9, 0x2d, 0x4e, 0xa3, 0x2e, 0xb5, 0xa4, 0xcb, 0x28, 0xfd, 0x20, 0x9f, 0xe4, 0x83, 0x3c, 0xad,
	0xdc, 0x9b, 0x27, 0x93, 0x87, 0x09, 0xf4, 0xa9, 0x88, 0x40, 0x3e, 0x73, 0x12, 0x25, 0x3e, 0x8d,
	0x92, 0xae, 0x70, 0x24, 0x6c, 0x5a, 0x8c, 0x57, 0x22, 0x6c, 0xc8, 0xd9, 0xbc, 0xaf, 0x9c, 0x42,
	0xec, 0x78, 0x7e, 0x44, 0xd8, 0x4c, 0xc9, 0x43, 0xc2, 0xf1, 0x26, 0x95, 0x75, 0x93, 0x8a, 0xa5,
	0x11, 0xf7, 0x43, 0x72, 0x4d, 0xf0, 0xe0, 0x6f, 0x82, 0xc4, 0xf1, 0x48, 0x88, 0xff, 0xd4, 0xb5,
	0x7f, 0x68, 0xe0, 0xf6, 0x80, 0x46, 0x9c, 0xd1, 0x20, 0x20, 0xec, 0x98, 0xc4, 0x01, 0x9d, 0x85,
	0x24, 0xe2, 0xfa, 0x7b, 0x50, 0x17, 0xe1, 0xc6, 0x98, 0x63, 0x43, 0x3b, 0xd2, 0x3a, 0x07, 0xfd,
	0x7b, 0x70, 0x35, 0x03, 0x16, 0x67, 0xa8, 0x4d, 0x0b, 0x36, 0x9c, 0xf6, 0xe0, 0x70, 0xf4, 0x91,
	0x38, 0xfc, 0x39, 0xe1, 0xd8, 0xd6, 0xe7, 0x8b, 0x56, 0x25, 0x5b, 0xb4, 0x80, 0xc2, 0xd0, 0xda,
	0x55, 0x27, 0x60, 0xd7, 0x23, 0x41, 0x68, 0xec, 0x48, 0xf7, 0xa7, 0x70, 0xab, 0x17, 0x0a, 0x4f,
	0x48, 0x10, 0x6e, 0x0a, 0x6e, 0xd7, 0xb3, 0x45, 0x6b, 0x57, 0xdc, 0x22, 0x69, 0xdf, 0xce, 0x34,
	0x60, 0x6c, 0x22, 0x9e, 0xf9, 0x09, 0xd7, 0xdf, 0x5e, 0x6b, 0x09, 0xff, 0xad, 0xa5, 0x50, 0xcb,
	0x8e, 0xff, 0xe7, 0x1d, 0xeb, 0x57, 0x48, 0xa1, 0xa1, 0x07, 0xf6, 0x7c, 0x4e, 0xc2, 0xc4, 0xd8,
	0x39, 0xaa, 0x76, 0x0e, 0xfa, 0x83, 0x2d, 0x2b, 0x6e, 0xac, 0xd7, 0xc8, 0xe7, 0xed, 0x9d, 0x0a,
	0x67, 0xb4, 0x1a, 0xd0, 0xfe, 0xb6, 0x03, 0x8c, 0x9b, 0x36, 0xa2, 0x77, 0x40, 0x9d, 0xe1, 0x4f,
	0x03, 0x0f, 0x33, 0x2e, 0x4b, 0x1e, 0xda, 0x87, 0x22, 0x30, 0xca, 0x31, 0xb4, 0xbe, 0xd5, 0x47,
	0xa0, 0x36, 0xc5, 0x41, 0x4a, 0x92, 0xfc, 0xa5, 0x3c, 0x29, 0x2c, 0x43, 0x7d, 0xe6, 0xef, 0xd6,
	0xbf, 0x03, 0x95, 0xb9, 0x44, 0x10, 0xe1, 0x9f, 0x9d, 0x0f, 0x5f, 0xd8, 0x20, 0x5b, 0xb4, 0x6a,
	0xaf, 0xa5, 0x23, 0xca, 0x9d, 0xf5, 0x14, 0x34, 0xa8, 0xe3, 0x23, 0x12, 0xd3, 0xc4, 0xe7, 0x94,
	0xcd, 0x8c, 0xaa, 0x1c, 0xf5, 0x68, 0xcb, 0xe5, 0x0c, 0x07, 0xa7, 0xca, 0xc3, 0xbe, 0x95, 0x2d,
	0x5a, 0x8d, 0x12, 0x84, 0xca, 0x53, 0xda, 0xdf, 0x35, 0x50, 0x26, 0xe8, 0x7d, 0x00, 0x98, 0x4a,
	0x21, 0x16, 0xb3, 0xaf, 0xbe, 0xd8, 0x82, 0x51, 0x81, 0xa5, 0xdf, 0x01, 0x55, 0x8e, 0x5d, 0xb9,
	0x9d, 0x7d, 0xfb, 0x20, 0x27, 0x57, 0x5f, 0x62, 0x17, 0x09, 0x5c, 0xbf, 0x0b, 0x6a, 0x63, 0xdf,
	0x25, 0x09, 0x97, 0xa5, 0xf6, 0xed, 0xff, 0x72, 0x46, 0xed, 0x58, 0xa2, 0x28, 0xbf, 0x15, 0x36,
	0x29, 0x0b, 0x8c, 0xdd, 0xb2, 0xcd, 0x2b, 0x74, 0x86, 0x04, 0x6e, 0x9f, 0xcf, 0x97, 0x66, 0xe5,
	0x62, 0x69, 0x56, 0x2e, 0x97, 0x66, 0xe5, 0x4b, 0x66, 0x6a, 0xf3, 0xcc, 0xd4, 0x2e, 0x32, 0x53,
	0xbb, 0xcc, 0x4c, 0xed, 0x67, 0x66, 0x6a, 0x5f, 0x7f, 0x99, 0x95, 0x37, 0xdd, 0xad, 0xfe, 0x40,
	0x7f, 0x07, 0x00, 0x00, 0xff, 0xff, 0xc1, 0x40, 0x0e, 0x6b, 0x70, 0x05, 0x00, 0x00,
}

func (m *ControllerDeployment) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ControllerDeployment) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ControllerDeployment) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Helm != nil {
		{
			size, err := m.Helm.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGenerated(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	{
		size, err := m.ObjectMeta.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintGenerated(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func (m *ControllerDeploymentList) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ControllerDeploymentList) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ControllerDeploymentList) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Items) > 0 {
		for iNdEx := len(m.Items) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Items[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenerated(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	{
		size, err := m.ListMeta.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintGenerated(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func (m *HelmControllerDeployment) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *HelmControllerDeployment) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *HelmControllerDeployment) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.OCIRepository != nil {
		{
			size, err := m.OCIRepository.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGenerated(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x1a
	}
	if m.Values != nil {
		{
			size, err := m.Values.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintGenerated(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	if m.RawChart != nil {
		i -= len(m.RawChart)
		copy(dAtA[i:], m.RawChart)
		i = encodeVarintGenerated(dAtA, i, uint64(len(m.RawChart)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *OCIRepository) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *OCIRepository) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *OCIRepository) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	i -= len(m.URL)
	copy(dAtA[i:], m.URL)
	i = encodeVarintGenerated(dAtA, i, uint64(len(m.URL)))
	i--
	dAtA[i] = 0x22
	i -= len(m.Digest)
	copy(dAtA[i:], m.Digest)
	i = encodeVarintGenerated(dAtA, i, uint64(len(m.Digest)))
	i--
	dAtA[i] = 0x1a
	i -= len(m.Tag)
	copy(dAtA[i:], m.Tag)
	i = encodeVarintGenerated(dAtA, i, uint64(len(m.Tag)))
	i--
	dAtA[i] = 0x12
	i -= len(m.Repository)
	copy(dAtA[i:], m.Repository)
	i = encodeVarintGenerated(dAtA, i, uint64(len(m.Repository)))
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func encodeVarintGenerated(dAtA []byte, offset int, v uint64) int {
	offset -= sovGenerated(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *ControllerDeployment) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.ObjectMeta.Size()
	n += 1 + l + sovGenerated(uint64(l))
	if m.Helm != nil {
		l = m.Helm.Size()
		n += 1 + l + sovGenerated(uint64(l))
	}
	return n
}

func (m *ControllerDeploymentList) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.ListMeta.Size()
	n += 1 + l + sovGenerated(uint64(l))
	if len(m.Items) > 0 {
		for _, e := range m.Items {
			l = e.Size()
			n += 1 + l + sovGenerated(uint64(l))
		}
	}
	return n
}

func (m *HelmControllerDeployment) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.RawChart != nil {
		l = len(m.RawChart)
		n += 1 + l + sovGenerated(uint64(l))
	}
	if m.Values != nil {
		l = m.Values.Size()
		n += 1 + l + sovGenerated(uint64(l))
	}
	if m.OCIRepository != nil {
		l = m.OCIRepository.Size()
		n += 1 + l + sovGenerated(uint64(l))
	}
	return n
}

func (m *OCIRepository) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Repository)
	n += 1 + l + sovGenerated(uint64(l))
	l = len(m.Tag)
	n += 1 + l + sovGenerated(uint64(l))
	l = len(m.Digest)
	n += 1 + l + sovGenerated(uint64(l))
	l = len(m.URL)
	n += 1 + l + sovGenerated(uint64(l))
	return n
}

func sovGenerated(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozGenerated(x uint64) (n int) {
	return sovGenerated(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *ControllerDeployment) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ControllerDeployment{`,
		`ObjectMeta:` + strings.Replace(strings.Replace(fmt.Sprintf("%v", this.ObjectMeta), "ObjectMeta", "v1.ObjectMeta", 1), `&`, ``, 1) + `,`,
		`Helm:` + strings.Replace(this.Helm.String(), "HelmControllerDeployment", "HelmControllerDeployment", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *ControllerDeploymentList) String() string {
	if this == nil {
		return "nil"
	}
	repeatedStringForItems := "[]ControllerDeployment{"
	for _, f := range this.Items {
		repeatedStringForItems += strings.Replace(strings.Replace(f.String(), "ControllerDeployment", "ControllerDeployment", 1), `&`, ``, 1) + ","
	}
	repeatedStringForItems += "}"
	s := strings.Join([]string{`&ControllerDeploymentList{`,
		`ListMeta:` + strings.Replace(strings.Replace(fmt.Sprintf("%v", this.ListMeta), "ListMeta", "v1.ListMeta", 1), `&`, ``, 1) + `,`,
		`Items:` + repeatedStringForItems + `,`,
		`}`,
	}, "")
	return s
}
func (this *HelmControllerDeployment) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&HelmControllerDeployment{`,
		`RawChart:` + valueToStringGenerated(this.RawChart) + `,`,
		`Values:` + strings.Replace(fmt.Sprintf("%v", this.Values), "JSON", "v11.JSON", 1) + `,`,
		`OCIRepository:` + strings.Replace(this.OCIRepository.String(), "OCIRepository", "OCIRepository", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *OCIRepository) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&OCIRepository{`,
		`Repository:` + fmt.Sprintf("%v", this.Repository) + `,`,
		`Tag:` + fmt.Sprintf("%v", this.Tag) + `,`,
		`Digest:` + fmt.Sprintf("%v", this.Digest) + `,`,
		`URL:` + fmt.Sprintf("%v", this.URL) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringGenerated(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *ControllerDeployment) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenerated
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ControllerDeployment: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ControllerDeployment: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ObjectMeta", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.ObjectMeta.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Helm", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Helm == nil {
				m.Helm = &HelmControllerDeployment{}
			}
			if err := m.Helm.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenerated(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenerated
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ControllerDeploymentList) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenerated
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ControllerDeploymentList: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ControllerDeploymentList: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ListMeta", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.ListMeta.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Items", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Items = append(m.Items, ControllerDeployment{})
			if err := m.Items[len(m.Items)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenerated(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenerated
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *HelmControllerDeployment) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenerated
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: HelmControllerDeployment: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HelmControllerDeployment: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field RawChart", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.RawChart = append(m.RawChart[:0], dAtA[iNdEx:postIndex]...)
			if m.RawChart == nil {
				m.RawChart = []byte{}
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Values", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Values == nil {
				m.Values = &v11.JSON{}
			}
			if err := m.Values.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field OCIRepository", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.OCIRepository == nil {
				m.OCIRepository = &OCIRepository{}
			}
			if err := m.OCIRepository.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenerated(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenerated
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *OCIRepository) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenerated
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: OCIRepository: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: OCIRepository: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Repository", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Repository = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Tag", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Tag = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Digest", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Digest = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field URL", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGenerated
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGenerated
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.URL = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenerated(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenerated
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipGenerated(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowGenerated
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGenerated
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthGenerated
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupGenerated
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthGenerated
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthGenerated        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowGenerated          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupGenerated = fmt.Errorf("proto: unexpected end of group")
)
