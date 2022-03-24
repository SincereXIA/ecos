// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: sun.proto

package sun

import (
	infos "ecos/edge-node/infos"
	common "ecos/messenger/common"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type RegisterResult struct {
	Result               *common.Result   `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
	RaftId               uint64           `protobuf:"varint,2,opt,name=raft_id,json=raftId,proto3" json:"raft_id,omitempty"`
	HasLeader            bool             `protobuf:"varint,3,opt,name=has_leader,json=hasLeader,proto3" json:"has_leader,omitempty"`
	GroupInfo            *infos.GroupInfo `protobuf:"bytes,4,opt,name=group_info,json=groupInfo,proto3" json:"group_info,omitempty"`
	XXX_NoUnkeyedLiteral struct{}         `json:"-"`
	XXX_unrecognized     []byte           `json:"-"`
	XXX_sizecache        int32            `json:"-"`
}

func (m *RegisterResult) Reset()         { *m = RegisterResult{} }
func (m *RegisterResult) String() string { return proto.CompactTextString(m) }
func (*RegisterResult) ProtoMessage()    {}
func (*RegisterResult) Descriptor() ([]byte, []int) {
	return fileDescriptor_df5d86f47d451473, []int{0}
}
func (m *RegisterResult) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RegisterResult) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_RegisterResult.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *RegisterResult) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RegisterResult.Merge(m, src)
}
func (m *RegisterResult) XXX_Size() int {
	return m.Size()
}
func (m *RegisterResult) XXX_DiscardUnknown() {
	xxx_messageInfo_RegisterResult.DiscardUnknown(m)
}

var xxx_messageInfo_RegisterResult proto.InternalMessageInfo

func (m *RegisterResult) GetResult() *common.Result {
	if m != nil {
		return m.Result
	}
	return nil
}

func (m *RegisterResult) GetRaftId() uint64 {
	if m != nil {
		return m.RaftId
	}
	return 0
}

func (m *RegisterResult) GetHasLeader() bool {
	if m != nil {
		return m.HasLeader
	}
	return false
}

func (m *RegisterResult) GetGroupInfo() *infos.GroupInfo {
	if m != nil {
		return m.GroupInfo
	}
	return nil
}

func init() {
	proto.RegisterType((*RegisterResult)(nil), "messenger.RegisterResult")
}

func init() { proto.RegisterFile("sun.proto", fileDescriptor_df5d86f47d451473) }

var fileDescriptor_df5d86f47d451473 = []byte{
	// 308 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x91, 0xcf, 0x4e, 0xc2, 0x40,
	0x10, 0xc6, 0xbb, 0x42, 0xd0, 0x8e, 0x08, 0xba, 0x9a, 0x58, 0x9b, 0xd8, 0x34, 0x3d, 0xe1, 0x05,
	0x12, 0x38, 0xea, 0xc1, 0x78, 0x21, 0x24, 0xea, 0x61, 0xbd, 0x79, 0x21, 0x95, 0x1d, 0x0a, 0x09,
	0xec, 0x90, 0xfd, 0xf3, 0x2e, 0x3e, 0x82, 0xcf, 0xe1, 0xc9, 0xa3, 0x8f, 0x60, 0xf0, 0x45, 0x4c,
	0xcb, 0x1f, 0x6b, 0xe4, 0x36, 0xfb, 0xcd, 0x37, 0xf3, 0xfd, 0x76, 0x17, 0x7c, 0xe3, 0x54, 0x7b,
	0xa1, 0xc9, 0x12, 0xf7, 0xe7, 0x68, 0x0c, 0xaa, 0x0c, 0x75, 0xd8, 0x54, 0x24, 0x71, 0x38, 0x55,
	0x63, 0x5a, 0xf5, 0xc2, 0xe3, 0x4c, 0x93, 0x5b, 0x94, 0x95, 0xfa, 0x88, 0xe6, 0x73, 0x5a, 0xcf,
	0x26, 0x6f, 0x0c, 0x1a, 0x02, 0xb3, 0xa9, 0xb1, 0xa8, 0x05, 0x1a, 0x37, 0xb3, 0xfc, 0x0a, 0x6a,
	0xba, 0xa8, 0x02, 0x16, 0xb3, 0xd6, 0x61, 0xf7, 0xa4, 0xbd, 0xdd, 0xdf, 0x5e, 0x59, 0xc4, 0xda,
	0xc0, 0xcf, 0x61, 0x5f, 0xa7, 0x63, 0x3b, 0x9c, 0xca, 0x60, 0x2f, 0x66, 0xad, 0xaa, 0xa8, 0xe5,
	0xc7, 0x81, 0xe4, 0x97, 0x00, 0x93, 0xd4, 0x0c, 0x67, 0x98, 0x4a, 0xd4, 0x41, 0x25, 0x66, 0xad,
	0x03, 0xe1, 0x4f, 0x52, 0x73, 0x5f, 0x08, 0xbc, 0x07, 0xf0, 0xcb, 0x15, 0x54, 0x8b, 0x98, 0xb3,
	0x52, 0x4c, 0x3f, 0x6f, 0x0e, 0xd4, 0x98, 0x84, 0x9f, 0x6d, 0xca, 0xee, 0x3b, 0x83, 0xca, 0x93,
	0x53, 0xfc, 0x16, 0xea, 0x0f, 0x44, 0x6a, 0x43, 0xcd, 0x4f, 0x4b, 0x83, 0x8f, 0x24, 0x31, 0x37,
	0x87, 0x17, 0x7f, 0xa0, 0xcb, 0xf7, 0x4b, 0x3c, 0x7e, 0x0d, 0x47, 0x7d, 0xb4, 0x2b, 0x96, 0xdc,
	0xbd, 0x7b, 0xc5, 0x2e, 0x31, 0xf1, 0xf8, 0x0d, 0x34, 0x05, 0x2e, 0x48, 0xdb, 0x2d, 0x24, 0xdf,
	0x89, 0x1e, 0xfe, 0x7f, 0xb7, 0xc4, 0xbb, 0x8b, 0x3f, 0x96, 0x11, 0xfb, 0x5c, 0x46, 0xec, 0x6b,
	0x19, 0xb1, 0xd7, 0xef, 0xc8, 0x7b, 0x6e, 0xe0, 0x88, 0x4c, 0x67, 0x34, 0x23, 0x27, 0x3b, 0xc6,
	0xa9, 0x97, 0x5a, 0xf1, 0x31, 0xbd, 0x9f, 0x00, 0x00, 0x00, 0xff, 0xff, 0x6e, 0xb6, 0x32, 0x07,
	0xe1, 0x01, 0x00, 0x00,
}

func (m *RegisterResult) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RegisterResult) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *RegisterResult) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.GroupInfo != nil {
		{
			size, err := m.GroupInfo.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintSun(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x22
	}
	if m.HasLeader {
		i--
		if m.HasLeader {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x18
	}
	if m.RaftId != 0 {
		i = encodeVarintSun(dAtA, i, uint64(m.RaftId))
		i--
		dAtA[i] = 0x10
	}
	if m.Result != nil {
		{
			size, err := m.Result.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintSun(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintSun(dAtA []byte, offset int, v uint64) int {
	offset -= sovSun(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *RegisterResult) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Result != nil {
		l = m.Result.Size()
		n += 1 + l + sovSun(uint64(l))
	}
	if m.RaftId != 0 {
		n += 1 + sovSun(uint64(m.RaftId))
	}
	if m.HasLeader {
		n += 2
	}
	if m.GroupInfo != nil {
		l = m.GroupInfo.Size()
		n += 1 + l + sovSun(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovSun(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozSun(x uint64) (n int) {
	return sovSun(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *RegisterResult) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSun
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
			return fmt.Errorf("proto: RegisterResult: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: RegisterResult: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Result", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSun
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
				return ErrInvalidLengthSun
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSun
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Result == nil {
				m.Result = &common.Result{}
			}
			if err := m.Result.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field RaftId", wireType)
			}
			m.RaftId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSun
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.RaftId |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field HasLeader", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSun
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.HasLeader = bool(v != 0)
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field GroupInfo", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSun
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
				return ErrInvalidLengthSun
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSun
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.GroupInfo == nil {
				m.GroupInfo = &infos.GroupInfo{}
			}
			if err := m.GroupInfo.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSun(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSun
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipSun(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowSun
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
					return 0, ErrIntOverflowSun
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
					return 0, ErrIntOverflowSun
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
				return 0, ErrInvalidLengthSun
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupSun
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthSun
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthSun        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowSun          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupSun = fmt.Errorf("proto: unexpected end of group")
)
