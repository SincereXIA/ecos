// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: history.proto

package node

import (
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

type History struct {
	HistoryMap           map[uint64]*GroupInfo `protobuf:"bytes,1,rep,name=historyMap,proto3" json:"historyMap,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *History) Reset()         { *m = History{} }
func (m *History) String() string { return proto.CompactTextString(m) }
func (*History) ProtoMessage()    {}
func (*History) Descriptor() ([]byte, []int) {
	return fileDescriptor_454388b49b309873, []int{0}
}
func (m *History) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *History) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_History.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *History) XXX_Merge(src proto.Message) {
	xxx_messageInfo_History.Merge(m, src)
}
func (m *History) XXX_Size() int {
	return m.Size()
}
func (m *History) XXX_DiscardUnknown() {
	xxx_messageInfo_History.DiscardUnknown(m)
}

var xxx_messageInfo_History proto.InternalMessageInfo

func (m *History) GetHistoryMap() map[uint64]*GroupInfo {
	if m != nil {
		return m.HistoryMap
	}
	return nil
}

func init() {
	proto.RegisterType((*History)(nil), "messenger.History")
	proto.RegisterMapType((map[uint64]*GroupInfo)(nil), "messenger.History.HistoryMapEntry")
}

func init() { proto.RegisterFile("history.proto", fileDescriptor_454388b49b309873) }

var fileDescriptor_454388b49b309873 = []byte{
	// 206 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xcd, 0xc8, 0x2c, 0x2e,
	0xc9, 0x2f, 0xaa, 0xd4, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0xcc, 0x4d, 0x2d, 0x2e, 0x4e,
	0xcd, 0x4b, 0x4f, 0x2d, 0x92, 0x12, 0x48, 0x2f, 0xca, 0x2f, 0x2d, 0x88, 0xcf, 0xcc, 0x4b, 0xcb,
	0x87, 0x48, 0x2a, 0x2d, 0x62, 0xe4, 0x62, 0xf7, 0x80, 0x28, 0x17, 0x72, 0xe2, 0xe2, 0x82, 0xea,
	0xf4, 0x4d, 0x2c, 0x90, 0x60, 0x54, 0x60, 0xd6, 0xe0, 0x36, 0x52, 0xd2, 0x83, 0xeb, 0xd6, 0x83,
	0xaa, 0x83, 0xd1, 0xbe, 0x89, 0x05, 0xae, 0x79, 0x25, 0x45, 0x95, 0x41, 0x48, 0xba, 0xa4, 0x82,
	0xb9, 0xf8, 0xd1, 0xa4, 0x85, 0x04, 0xb8, 0x98, 0xb3, 0x53, 0x2b, 0x25, 0x18, 0x15, 0x18, 0x35,
	0x58, 0x82, 0x40, 0x4c, 0x21, 0x2d, 0x2e, 0xd6, 0xb2, 0xc4, 0x9c, 0xd2, 0x54, 0x09, 0x26, 0x05,
	0x46, 0x0d, 0x6e, 0x23, 0x11, 0x24, 0x3b, 0xdc, 0x41, 0x0e, 0xf4, 0xcc, 0x4b, 0xcb, 0x0f, 0x82,
	0x28, 0xb1, 0x62, 0xb2, 0x60, 0x74, 0x52, 0x3d, 0xf1, 0x48, 0x8e, 0xf1, 0xc2, 0x23, 0x39, 0xc6,
	0x07, 0x8f, 0xe4, 0x18, 0x67, 0x3c, 0x96, 0x63, 0x88, 0x12, 0x4e, 0x4d, 0xce, 0x2f, 0xd6, 0x4f,
	0x4d, 0x49, 0x4f, 0xd5, 0xcd, 0xcb, 0x4f, 0x49, 0xd5, 0x07, 0x11, 0x49, 0x6c, 0x60, 0x2f, 0x19,
	0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0xca, 0x05, 0x6f, 0x8a, 0x00, 0x01, 0x00, 0x00,
}

func (m *History) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *History) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *History) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.HistoryMap) > 0 {
		for k := range m.HistoryMap {
			v := m.HistoryMap[k]
			baseI := i
			if v != nil {
				{
					size, err := v.MarshalToSizedBuffer(dAtA[:i])
					if err != nil {
						return 0, err
					}
					i -= size
					i = encodeVarintHistory(dAtA, i, uint64(size))
				}
				i--
				dAtA[i] = 0x12
			}
			i = encodeVarintHistory(dAtA, i, uint64(k))
			i--
			dAtA[i] = 0x8
			i = encodeVarintHistory(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintHistory(dAtA []byte, offset int, v uint64) int {
	offset -= sovHistory(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *History) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.HistoryMap) > 0 {
		for k, v := range m.HistoryMap {
			_ = k
			_ = v
			l = 0
			if v != nil {
				l = v.Size()
				l += 1 + sovHistory(uint64(l))
			}
			mapEntrySize := 1 + sovHistory(uint64(k)) + l
			n += mapEntrySize + 1 + sovHistory(uint64(mapEntrySize))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovHistory(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozHistory(x uint64) (n int) {
	return sovHistory(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *History) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowHistory
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
			return fmt.Errorf("proto: History: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: History: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field HistoryMap", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowHistory
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
				return ErrInvalidLengthHistory
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthHistory
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.HistoryMap == nil {
				m.HistoryMap = make(map[uint64]*GroupInfo)
			}
			var mapkey uint64
			var mapvalue *GroupInfo
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowHistory
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
							return ErrIntOverflowHistory
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
							return ErrIntOverflowHistory
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
						return ErrInvalidLengthHistory
					}
					postmsgIndex := iNdEx + mapmsglen
					if postmsgIndex < 0 {
						return ErrInvalidLengthHistory
					}
					if postmsgIndex > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = &GroupInfo{}
					if err := mapvalue.Unmarshal(dAtA[iNdEx:postmsgIndex]); err != nil {
						return err
					}
					iNdEx = postmsgIndex
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipHistory(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if (skippy < 0) || (iNdEx+skippy) < 0 {
						return ErrInvalidLengthHistory
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.HistoryMap[mapkey] = mapvalue
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipHistory(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthHistory
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
func skipHistory(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowHistory
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
					return 0, ErrIntOverflowHistory
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
					return 0, ErrIntOverflowHistory
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
				return 0, ErrInvalidLengthHistory
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupHistory
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthHistory
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthHistory        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowHistory          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupHistory = fmt.Errorf("proto: unexpected end of group")
)