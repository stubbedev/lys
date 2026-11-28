package ptp

import (
	"slices"
	"time"
)

// DeviceInfo is the dataset GetDeviceInfo returns.
type DeviceInfo struct {
	StandardVersion        uint16
	VendorExtensionID      uint32
	VendorExtensionVersion uint16
	VendorExtensionDesc    string
	FunctionalMode         uint16
	Operations             []OpCode
	Events                 []EventCode
	DeviceProperties       []uint16
	CaptureFormats         []ObjectFormat
	ImageFormats           []ObjectFormat
	Manufacturer           string
	Model                  string
	DeviceVersion          string
	SerialNumber           string
}

// Supports is whether the device lists op among its operations.
func (d DeviceInfo) Supports(op OpCode) bool { return slices.Contains(d.Operations, op) }

func convert[T, F ~uint16 | ~uint32](v []F) []T {
	out := make([]T, len(v))
	for i, x := range v {
		out[i] = T(x)
	}
	return out
}

// ParseDeviceInfo decodes a DeviceInfo dataset.
func ParseDeviceInfo(b []byte) (DeviceInfo, error) {
	d := decoder{b: b}
	info := DeviceInfo{
		StandardVersion:        d.u16(),
		VendorExtensionID:      d.u32(),
		VendorExtensionVersion: d.u16(),
		VendorExtensionDesc:    d.str(),
		FunctionalMode:         d.u16(),
		Operations:             convert[OpCode](d.u16s()),
		Events:                 convert[EventCode](d.u16s()),
		DeviceProperties:       d.u16s(),
		CaptureFormats:         convert[ObjectFormat](d.u16s()),
		ImageFormats:           convert[ObjectFormat](d.u16s()),
		Manufacturer:           d.str(),
		Model:                  d.str(),
		DeviceVersion:          d.str(),
		SerialNumber:           d.str(),
	}
	return info, d.done("DeviceInfo")
}

// Append appends the dataset's encoding to b.
func (d DeviceInfo) Append(b []byte) ([]byte, error) {
	b = appendU16(b, d.StandardVersion)
	b = appendU32(b, d.VendorExtensionID)
	b = appendU16(b, d.VendorExtensionVersion)
	b, err := AppendString(b, d.VendorExtensionDesc)
	if err != nil {
		return b, err
	}
	b = appendU16(b, d.FunctionalMode)
	b = appendU16s(b, convert[uint16](d.Operations))
	b = appendU16s(b, convert[uint16](d.Events))
	b = appendU16s(b, d.DeviceProperties)
	b = appendU16s(b, convert[uint16](d.CaptureFormats))
	b = appendU16s(b, convert[uint16](d.ImageFormats))
	for _, s := range []string{d.Manufacturer, d.Model, d.DeviceVersion, d.SerialNumber} {
		if b, err = AppendString(b, s); err != nil {
			return b, err
		}
	}
	return b, nil
}

// Access is what a store lets the host do.
type Access uint16

// The access capabilities of a store.
const (
	ReadWrite            Access = 0
	ReadOnly             Access = 1
	ReadOnlyWithDeletion Access = 2
)

// StorageInfo is the dataset GetStorageInfo returns.
type StorageInfo struct {
	StorageType        uint16
	FilesystemType     uint16
	Access             Access
	MaxCapacity        uint64
	FreeSpace          uint64
	FreeSpaceInObjects uint32
	Description        string
	VolumeLabel        string
}

// ParseStorageInfo decodes a StorageInfo dataset.
func ParseStorageInfo(b []byte) (StorageInfo, error) {
	d := decoder{b: b}
	info := StorageInfo{
		StorageType:        d.u16(),
		FilesystemType:     d.u16(),
		Access:             Access(d.u16()),
		MaxCapacity:        d.u64(),
		FreeSpace:          d.u64(),
		FreeSpaceInObjects: d.u32(),
		Description:        d.str(),
		VolumeLabel:        d.str(),
	}
	return info, d.done("StorageInfo")
}

// Append appends the dataset's encoding to b.
func (s StorageInfo) Append(b []byte) ([]byte, error) {
	b = appendU16(b, s.StorageType)
	b = appendU16(b, s.FilesystemType)
	b = appendU16(b, uint16(s.Access))
	b = appendU64(b, s.MaxCapacity)
	b = appendU64(b, s.FreeSpace)
	b = appendU32(b, s.FreeSpaceInObjects)
	b, err := AppendString(b, s.Description)
	if err != nil {
		return b, err
	}
	return AppendString(b, s.VolumeLabel)
}

// SizeUnknown is the 32-bit compressed size of an object of 4 GiB or
// more, whose size only the ObjectSize property gives.
const SizeUnknown uint32 = 0xFFFFFFFF

// ObjectInfo is the dataset GetObjectInfo returns and SendObjectInfo
// sends. Its dates are wall clock in the location the Conn was opened
// with when the device names no zone.
type ObjectInfo struct {
	StorageID        StorageID
	Format           ObjectFormat
	ProtectionStatus uint16
	CompressedSize   uint32
	ThumbFormat      ObjectFormat
	ThumbSize        uint32
	ThumbWidth       uint32
	ThumbHeight      uint32
	ImageWidth       uint32
	ImageHeight      uint32
	ImageBitDepth    uint32
	Parent           ObjectHandle
	AssociationType  uint16
	AssociationDesc  uint32
	SequenceNumber   uint32
	Filename         string
	CaptureDate      time.Time
	ModificationDate time.Time
	Keywords         string
}

// ParseObjectInfo decodes an ObjectInfo dataset, reading dates without
// a zone in loc.
func ParseObjectInfo(b []byte, loc *time.Location) (ObjectInfo, error) {
	d := decoder{b: b}
	info := ObjectInfo{
		StorageID:        StorageID(d.u32()),
		Format:           ObjectFormat(d.u16()),
		ProtectionStatus: d.u16(),
		CompressedSize:   d.u32(),
		ThumbFormat:      ObjectFormat(d.u16()),
		ThumbSize:        d.u32(),
		ThumbWidth:       d.u32(),
		ThumbHeight:      d.u32(),
		ImageWidth:       d.u32(),
		ImageHeight:      d.u32(),
		ImageBitDepth:    d.u32(),
		Parent:           ObjectHandle(d.u32()),
		AssociationType:  d.u16(),
		AssociationDesc:  d.u32(),
		SequenceNumber:   d.u32(),
		Filename:         d.str(),
		CaptureDate:      d.date(loc),
		ModificationDate: d.date(loc),
		Keywords:         d.str(),
	}
	return info, d.done("ObjectInfo")
}

// Append appends the dataset's encoding to b, writing dates in loc.
func (o ObjectInfo) Append(b []byte, loc *time.Location) ([]byte, error) {
	b = appendU32(b, uint32(o.StorageID))
	b = appendU16(b, uint16(o.Format))
	b = appendU16(b, o.ProtectionStatus)
	b = appendU32(b, o.CompressedSize)
	b = appendU16(b, uint16(o.ThumbFormat))
	for _, v := range []uint32{o.ThumbSize, o.ThumbWidth, o.ThumbHeight, o.ImageWidth, o.ImageHeight, o.ImageBitDepth, uint32(o.Parent)} {
		b = appendU32(b, v)
	}
	b = appendU16(b, o.AssociationType)
	b = appendU32(b, o.AssociationDesc)
	b = appendU32(b, o.SequenceNumber)
	var err error
	for _, s := range []string{o.Filename, FormatDate(o.CaptureDate, loc), FormatDate(o.ModificationDate, loc), o.Keywords} {
		if b, err = AppendString(b, s); err != nil {
			return b, err
		}
	}
	return b, nil
}

// ParseArray decodes an array of 32-bit values, the ObjectHandle or
// StorageID array an operation returns.
func ParseArray[T ~uint32](b []byte) ([]T, error) {
	d := decoder{b: b}
	v := convert[T](d.u32s())
	return v, d.done("array")
}

// AppendArray appends the encoding of an array of 32-bit values to b.
func AppendArray[T ~uint32](b []byte, v []T) []byte { return appendU32s(b, convert[uint32](v)) }

// ParseUint64 decodes a UINT64 property value.
func ParseUint64(b []byte) (uint64, error) {
	d := decoder{b: b}
	v := d.u64()
	return v, d.done("UINT64 value")
}

// AppendUint64 appends the encoding of a UINT64 property value to b.
func AppendUint64(b []byte, v uint64) []byte { return appendU64(b, v) }

// ParseString decodes a string property value.
func ParseString(b []byte) (string, error) {
	d := decoder{b: b}
	v := d.str()
	return v, d.done("string value")
}
