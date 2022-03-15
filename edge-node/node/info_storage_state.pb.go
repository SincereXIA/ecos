// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: info_storage_state.proto

package node

import (
	timestamppb "ecos/messenger/timestamppb"
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

type InfoStorageState struct {
	Term                 uint64                 `protobuf:"varint,1,opt,name=Term,proto3" json:"Term,omitempty"`
	LeaderID             uint64                 `protobuf:"varint,2,opt,name=LeaderID,proto3" json:"LeaderID,omitempty"`
	InfoMap              map[uint64]*NodeInfo   `protobuf:"bytes,3,rep,name=InfoMap,proto3" json:"InfoMap,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	UpdateTimeStamp      *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=UpdateTimeStamp,proto3" json:"UpdateTimeStamp,omitempty"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *InfoStorageState) Reset()         { *m = InfoStorageState{} }
func (m *InfoStorageState) String() string { return proto.CompactTextString(m) }
func (*InfoStorageState) ProtoMessage()    {}
func (*InfoStorageState) Descriptor() ([]byte, []int) {
	return fileDescriptor_da073a8bbbf8c475, []int{0}
}
func (m *InfoStorageState) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *InfoStorageState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_InfoStorageState.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *InfoStorageState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InfoStorageState.Merge(m, src)
}
func (m *InfoStorageState) XXX_Size() int {
	return m.Size()
}
func (m *InfoStorageState) XXX_DiscardUnknown() {
	xxx_messageInfo_InfoStorageState.DiscardUnknown(m)
}

var xxx_messageInfo_InfoStorageState proto.InternalMessageInfo

func (m *InfoStorageState) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *InfoStorageState) GetLeaderID() uint64 {
	if m != nil {
		return m.LeaderID
	}
	return 0
}

func (m *InfoStorageState) GetInfoMap() map[uint64]*NodeInfo {
	if m != nil {
		return m.InfoMap
	}
	return nil
}

func (m *InfoStorageState) GetUpdateTimeStamp() *timestamppb.Timestamp {
	if m != nil {
		return m.UpdateTimeStamp
	}
	return nil
}

func init() {
	proto.RegisterType((*InfoStorageState)(nil), "messenger.InfoStorageState")
	proto.RegisterMapType((map[uint64]*NodeInfo)(nil), "messenger.InfoStorageState.InfoMapEntry")
}

func init() { proto.RegisterFile("info_storage_state.proto", fileDescriptor_da073a8bbbf8c475) }

var fileDescriptor_da073a8bbbf8c475 = []byte{
	// 291 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0xc8, 0xcc, 0x4b, 0xcb,
	0x8f, 0x2f, 0x2e, 0xc9, 0x2f, 0x4a, 0x4c, 0x4f, 0x8d, 0x2f, 0x2e, 0x49, 0x2c, 0x49, 0xd5, 0x2b,
	0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0xcc, 0x4d, 0x2d, 0x2e, 0x4e, 0xcd, 0x4b, 0x4f, 0x2d, 0x92,
	0xe2, 0xcf, 0xcb, 0x4f, 0x49, 0x8d, 0x07, 0xa9, 0x84, 0xc8, 0x49, 0xf1, 0x97, 0x64, 0xe6, 0xa6,
	0x16, 0x97, 0x24, 0xe6, 0x16, 0x40, 0x04, 0x94, 0xa6, 0x33, 0x71, 0x09, 0x78, 0xe6, 0xa5, 0xe5,
	0x07, 0x43, 0x0c, 0x0a, 0x06, 0x99, 0x23, 0x24, 0xc4, 0xc5, 0x12, 0x92, 0x5a, 0x94, 0x2b, 0xc1,
	0xa8, 0xc0, 0xa8, 0xc1, 0x12, 0x04, 0x66, 0x0b, 0x49, 0x71, 0x71, 0xf8, 0xa4, 0x26, 0xa6, 0xa4,
	0x16, 0x79, 0xba, 0x48, 0x30, 0x81, 0xc5, 0xe1, 0x7c, 0x21, 0x27, 0x2e, 0x76, 0x90, 0x19, 0xbe,
	0x89, 0x05, 0x12, 0xcc, 0x0a, 0xcc, 0x1a, 0xdc, 0x46, 0x1a, 0x7a, 0x70, 0x37, 0xe8, 0xa1, 0x9b,
	0xae, 0x07, 0x55, 0xea, 0x9a, 0x57, 0x52, 0x54, 0x19, 0x04, 0xd3, 0x28, 0x64, 0xc7, 0xc5, 0x1f,
	0x5a, 0x90, 0x92, 0x58, 0x92, 0x1a, 0x92, 0x99, 0x0b, 0x52, 0x98, 0x5b, 0x20, 0xc1, 0xa2, 0xc0,
	0xa8, 0xc1, 0x6d, 0x24, 0x82, 0x64, 0x56, 0x08, 0xcc, 0xf5, 0x41, 0xe8, 0x8a, 0xa5, 0xfc, 0xb9,
	0x78, 0x90, 0x0d, 0x16, 0x12, 0xe0, 0x62, 0xce, 0x4e, 0xad, 0x84, 0x7a, 0x01, 0xc4, 0x14, 0xd2,
	0xe4, 0x62, 0x2d, 0x4b, 0xcc, 0x29, 0x4d, 0x05, 0x3b, 0x9f, 0xdb, 0x48, 0x18, 0xc9, 0x5c, 0xbf,
	0xfc, 0x94, 0x54, 0x90, 0xee, 0x20, 0x88, 0x0a, 0x2b, 0x26, 0x0b, 0x46, 0x27, 0xd5, 0x13, 0x8f,
	0xe4, 0x18, 0x2f, 0x3c, 0x92, 0x63, 0x7c, 0xf0, 0x48, 0x8e, 0x71, 0xc6, 0x63, 0x39, 0x86, 0x28,
	0xe1, 0xd4, 0xe4, 0xfc, 0x62, 0xfd, 0xd4, 0x94, 0xf4, 0x54, 0x5d, 0x50, 0xb8, 0xea, 0x83, 0x88,
	0x24, 0x36, 0x70, 0x38, 0x1a, 0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0xaf, 0x1e, 0x98, 0x64, 0x90,
	0x01, 0x00, 0x00,
}

func (m *InfoStorageState) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *InfoStorageState) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *InfoStorageState) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.UpdateTimeStamp != nil {
		{
			size, err := m.UpdateTimeStamp.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintInfoStorageState(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x22
	}
	if len(m.InfoMap) > 0 {
		for k := range m.InfoMap {
			v := m.InfoMap[k]
			baseI := i
			if v != nil {
				{
					size, err := v.MarshalToSizedBuffer(dAtA[:i])
					if err != nil {
						return 0, err
					}
					i -= size
					i = encodeVarintInfoStorageState(dAtA, i, uint64(size))
				}
				i--
				dAtA[i] = 0x12
			}
			i = encodeVarintInfoStorageState(dAtA, i, uint64(k))
			i--
			dAtA[i] = 0x8
			i = encodeVarintInfoStorageState(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0x1a
		}
	}
	if m.LeaderID != 0 {
		i = encodeVarintInfoStorageState(dAtA, i, uint64(m.LeaderID))
		i--
		dAtA[i] = 0x10
	}
	if m.Term != 0 {
		i = encodeVarintInfoStorageState(dAtA, i, uint64(m.Term))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintInfoStorageState(dAtA []byte, offset int, v uint64) int {
	offset -= sovInfoStorageState(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *InfoStorageState) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Term != 0 {
		n += 1 + sovInfoStorageState(uint64(m.Term))
	}
	if m.LeaderID != 0 {
		n += 1 + sovInfoStorageState(uint64(m.LeaderID))
	}
	if len(m.InfoMap) > 0 {
		for k, v := range m.InfoMap {
			_ = k
			_ = v
			l = 0
			if v != nil {
				l = v.Size()
				l += 1 + sovInfoStorageState(uint64(l))
			}
			mapEntrySize := 1 + sovInfoStorageState(uint64(k)) + l
			n += mapEntrySize + 1 + sovInfoStorageState(uint64(mapEntrySize))
		}
	}
	if m.UpdateTimeStamp != nil {
		l = m.UpdateTimeStamp.Size()
		n += 1 + l + sovInfoStorageState(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovInfoStorageState(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozInfoStorageState(x uint64) (n int) {
	return sovInfoStorageState(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *InfoStorageState) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowInfoStorageState
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
			return fmt.Errorf("proto: InfoStorageState: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: InfoStorageState: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Term", wireType)
			}
			m.Term = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInfoStorageState
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Term |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field LeaderID", wireType)
			}
			m.LeaderID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInfoStorageState
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.LeaderID |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field InfoMap", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInfoStorageState
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
				return ErrInvalidLengthInfoStorageState
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInfoStorageState
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.InfoMap == nil {
				m.InfoMap = make(map[uint64]*NodeInfo)
			}
			var mapkey uint64
			var mapvalue *NodeInfo
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowInfoStorageState
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
				if fieldNum == 1 {
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowInfoStorageState
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						mapkey |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
				} else if fieldNum == 2 {
					var mapmsglen int
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowInfoStorageState
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						mapmsglen |= int(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					if mapmsglen < 0 {
						return ErrInvalidLengthInfoStorageState
					}
					postmsgIndex := iNdEx + mapmsglen
					if postmsgIndex < 0 {
						return ErrInvalidLengthInfoStorageState
					}
					if postmsgIndex > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = &NodeInfo{}
					if err := mapvalue.Unmarshal(dAtA[iNdEx:postmsgIndex]); err != nil {
						return err
					}
					iNdEx = postmsgIndex
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipInfoStorageState(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if (skippy < 0) || (iNdEx+skippy) < 0 {
						return ErrInvalidLengthInfoStorageState
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.InfoMap[mapkey] = mapvalue
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field UpdateTimeStamp", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInfoStorageState
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
				return ErrInvalidLengthInfoStorageState
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInfoStorageState
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.UpdateTimeStamp == nil {
				m.UpdateTimeStamp = &timestamppb.Timestamp{}
			}
			if err := m.UpdateTimeStamp.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipInfoStorageState(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthInfoStorageState
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
func skipInfoStorageState(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowInfoStorageState
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
					return 0, ErrIntOverflowInfoStorageState
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
					return 0, ErrIntOverflowInfoStorageState
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
				return 0, ErrInvalidLengthInfoStorageState
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupInfoStorageState
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthInfoStorageState
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthInfoStorageState        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowInfoStorageState          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupInfoStorageState = fmt.Errorf("proto: unexpected end of group")
)