package ptp

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStringsRoundTripAsUTF16(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "DCIM", "Ærøskøbing 📷.jpg"} {
		b, err := AppendString(nil, s)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseString(b)
		if err != nil || got != s {
			t.Errorf("%q came back as %q, %v", s, got, err)
		}
	}
	if b, _ := AppendString(nil, ""); len(b) != 1 || b[0] != 0 {
		t.Errorf("the empty string is %x, want a zero count", b)
	}
}

func TestAStringBeyondTheCountIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := AppendString(nil, strings.Repeat("a", 254)); err != nil {
		t.Fatalf("254 units with the NUL fit in a count: %v", err)
	}
	if _, err := AppendString(nil, strings.Repeat("a", 255)); err == nil {
		t.Fatal("255 units with the NUL overflow the count, and were taken")
	}
}

func TestParseDateReadsEveryForm(t *testing.T) {
	t.Parallel()
	berlin := time.FixedZone("CET", 3600)
	cases := map[string]time.Time{
		"20241005T143000":        time.Date(2024, 10, 5, 14, 30, 0, 0, berlin),
		"20241005T143000.5":      time.Date(2024, 10, 5, 14, 30, 0, 5e8, berlin),
		"20241005T143000Z":       time.Date(2024, 10, 5, 14, 30, 0, 0, time.UTC),
		"20241005T143000+0200":   time.Date(2024, 10, 5, 12, 30, 0, 0, time.UTC),
		"20241005T143000.1-0130": time.Date(2024, 10, 5, 16, 0, 0, 1e8, time.UTC),
		"":                       {},
	}
	for s, want := range cases {
		got, err := ParseDate(s, berlin)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseDate(%q) = %v, %v; want %v", s, got, err, want)
		}
	}
	for _, s := range []string{"2024", "20241005T143000junk", "20241305T143000", "20241005T143000+02"} {
		if _, err := ParseDate(s, berlin); !errors.Is(err, ErrProtocol) {
			t.Errorf("ParseDate(%q) answered %v, want ErrProtocol", s, err)
		}
	}
}

func TestFormatDateWritesWallClockInTheLocation(t *testing.T) {
	t.Parallel()
	if s := FormatDate(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), time.FixedZone("", 3600)); s != "20240102T040405" {
		t.Fatalf("FormatDate wrote %q", s)
	}
	if s := FormatDate(time.Time{}, time.UTC); s != "" {
		t.Fatalf("the zero time is %q", s)
	}
}

func TestDatasetsRoundTrip(t *testing.T) {
	t.Parallel()
	device := DeviceInfo{
		StandardVersion: 100, VendorExtensionID: 6, VendorExtensionVersion: 100,
		VendorExtensionDesc: "microsoft.com: 1.0;", Operations: []OpCode{OpGetDeviceInfo, OpGetObject},
		Events: []EventCode{EventObjectAdded}, DeviceProperties: []uint16{0xD402}, CaptureFormats: []ObjectFormat{},
		ImageFormats: []ObjectFormat{FormatUndefined}, Manufacturer: "Google", Model: "Pixel 7", DeviceVersion: "1.0", SerialNumber: "123",
	}
	b, err := device.Append(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ParseDeviceInfo(b); err != nil || !reflect.DeepEqual(got, device) {
		t.Errorf("DeviceInfo came back as %+v, %v", got, err)
	}
	storage := StorageInfo{StorageType: 3, FilesystemType: 2, Access: ReadOnly, MaxCapacity: 1 << 40, FreeSpace: 1 << 30, Description: "Internal shared storage"}
	if b, err = storage.Append(nil); err != nil {
		t.Fatal(err)
	}
	if got, err := ParseStorageInfo(b); err != nil || got != storage {
		t.Errorf("StorageInfo came back as %+v, %v", got, err)
	}
	object := ObjectInfo{StorageID: 0x10001, Format: FormatUndefined, CompressedSize: 42, Parent: 7, Filename: "a.jpg", ModificationDate: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}
	if b, err = object.Append(nil, time.UTC); err != nil {
		t.Fatal(err)
	}
	if got, err := ParseObjectInfo(b, time.UTC); err != nil || got != object {
		t.Errorf("ObjectInfo came back as %+v, %v", got, err)
	}
	if _, err := ParseObjectInfo(b[:len(b)-3], time.UTC); !errors.Is(err, ErrProtocol) {
		t.Errorf("a truncated ObjectInfo answered %v", err)
	}
}

func TestAnArrayLongerThanItsBytesIsRefused(t *testing.T) {
	t.Parallel()
	b := appendU32(nil, 1000)
	if _, err := ParseArray[ObjectHandle](b); !errors.Is(err, ErrProtocol) {
		t.Fatalf("a 1000 element array in 4 bytes answered %v", err)
	}
	if got, err := ParseArray[ObjectHandle](AppendArray(nil, []ObjectHandle{1, 2})); err != nil || !reflect.DeepEqual(got, []ObjectHandle{1, 2}) {
		t.Fatalf("an array came back as %v, %v", got, err)
	}
}

func TestParseParamsChecksTheLength(t *testing.T) {
	t.Parallel()
	b := AppendParams(nil, TypeResponse, uint16(RespOK), 3, []uint32{1, 2, 3})
	h, err := ParseHeader(b)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ParseParams(b, h.Length); err != nil || !reflect.DeepEqual(got, []uint32{1, 2, 3}) {
		t.Fatalf("ParseParams = %v, %v", got, err)
	}
	padded := append(b, make([]byte, 64)...)
	for _, length := range []uint32{h.Length - 1, HeaderSize + 4*(MaxParams+1)} {
		if _, err := ParseParams(padded, length); !errors.Is(err, ErrProtocol) {
			t.Errorf("ParseParams of length %d answered %v", length, err)
		}
	}
	if _, err := ParseParams(b, h.Length+4); !errors.Is(err, ErrProtocol) {
		t.Errorf("ParseParams past the bytes answered %v", err)
	}
	if _, err := ParseHeader(b[:8]); !errors.Is(err, ErrProtocol) {
		t.Errorf("a short header answered %v", err)
	}
}

func TestResponseErrorsMatchTheStandardErrors(t *testing.T) {
	t.Parallel()
	cases := map[ResponseCode]error{
		RespInvalidObjectHandle:   fs.ErrNotExist,
		RespInvalidParentObject:   fs.ErrNotExist,
		RespStoreReadOnly:         fs.ErrPermission,
		RespObjectWriteProtected:  fs.ErrPermission,
		RespOperationNotSupported: errors.ErrUnsupported,
		RespStoreFull:             ErrStoreFull,
		RespDeviceBusy:            ErrBusy,
	}
	for code, target := range cases {
		err := error(&ResponseError{Op: OpGetObject, Code: code})
		if !errors.Is(err, target) {
			t.Errorf("%v is not %v", code, target)
		}
	}
	if err := error(&ResponseError{Op: OpGetObject, Code: RespGeneralError}); errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		t.Error("GeneralError matched a specific error")
	}
	if s := (&ResponseError{Op: OpGetObject, Code: RespStoreFull}).Error(); s != "ptp: GetObject: StoreFull" {
		t.Errorf("the error reads %q", s)
	}
}
