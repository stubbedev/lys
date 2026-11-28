// Package ptp speaks the Picture Transfer Protocol (ISO 15740) and the
// Media Transfer Protocol that extends it over a Transport: the
// containers, the datasets and a Conn that runs one session's
// transactions in order.
package ptp

import "fmt"

// OpCode is the operation a command container names.
type OpCode uint16

// The operations a Conn runs.
const (
	OpGetDeviceInfo      OpCode = 0x1001
	OpOpenSession        OpCode = 0x1002
	OpCloseSession       OpCode = 0x1003
	OpGetStorageIDs      OpCode = 0x1004
	OpGetStorageInfo     OpCode = 0x1005
	OpGetObjectHandles   OpCode = 0x1007
	OpGetObjectInfo      OpCode = 0x1008
	OpGetObject          OpCode = 0x1009
	OpDeleteObject       OpCode = 0x100B
	OpSendObjectInfo     OpCode = 0x100C
	OpSendObject         OpCode = 0x100D
	OpMoveObject         OpCode = 0x1019
	OpGetPartialObject   OpCode = 0x101B
	OpGetPartialObject64 OpCode = 0x95C1
	OpGetObjectPropValue OpCode = 0x9803
	OpSetObjectPropValue OpCode = 0x9804
)

var opNames = map[OpCode]string{
	OpGetDeviceInfo:      "GetDeviceInfo",
	OpOpenSession:        "OpenSession",
	OpCloseSession:       "CloseSession",
	OpGetStorageIDs:      "GetStorageIDs",
	OpGetStorageInfo:     "GetStorageInfo",
	OpGetObjectHandles:   "GetObjectHandles",
	OpGetObjectInfo:      "GetObjectInfo",
	OpGetObject:          "GetObject",
	OpDeleteObject:       "DeleteObject",
	OpSendObjectInfo:     "SendObjectInfo",
	OpSendObject:         "SendObject",
	OpMoveObject:         "MoveObject",
	OpGetPartialObject:   "GetPartialObject",
	OpGetPartialObject64: "GetPartialObject64",
	OpGetObjectPropValue: "GetObjectPropValue",
	OpSetObjectPropValue: "SetObjectPropValue",
}

func (o OpCode) String() string {
	if name, ok := opNames[o]; ok {
		return name
	}
	return fmt.Sprintf("operation 0x%04x", uint16(o))
}

// ResponseCode is the outcome a response container reports.
type ResponseCode uint16

// The responses told apart.
const (
	RespOK                    ResponseCode = 0x2001
	RespGeneralError          ResponseCode = 0x2002
	RespSessionNotOpen        ResponseCode = 0x2003
	RespOperationNotSupported ResponseCode = 0x2005
	RespParameterNotSupported ResponseCode = 0x2006
	RespIncompleteTransfer    ResponseCode = 0x2007
	RespInvalidStorageID      ResponseCode = 0x2008
	RespInvalidObjectHandle   ResponseCode = 0x2009
	RespStoreFull             ResponseCode = 0x200C
	RespObjectWriteProtected  ResponseCode = 0x200D
	RespStoreReadOnly         ResponseCode = 0x200E
	RespAccessDenied          ResponseCode = 0x200F
	RespStoreNotAvailable     ResponseCode = 0x2013
	RespDeviceBusy            ResponseCode = 0x2019
	RespInvalidParentObject   ResponseCode = 0x201A
	RespInvalidParameter      ResponseCode = 0x201D
	RespSessionAlreadyOpen    ResponseCode = 0x201E
	RespTransactionCancelled  ResponseCode = 0x201F
	RespInvalidObjectPropCode ResponseCode = 0xA801
	RespObjectTooLarge        ResponseCode = 0xA809
)

var respNames = map[ResponseCode]string{
	RespOK:                    "OK",
	RespGeneralError:          "GeneralError",
	RespSessionNotOpen:        "SessionNotOpen",
	RespOperationNotSupported: "OperationNotSupported",
	RespParameterNotSupported: "ParameterNotSupported",
	RespIncompleteTransfer:    "IncompleteTransfer",
	RespInvalidStorageID:      "InvalidStorageID",
	RespInvalidObjectHandle:   "InvalidObjectHandle",
	RespStoreFull:             "StoreFull",
	RespObjectWriteProtected:  "ObjectWriteProtected",
	RespStoreReadOnly:         "StoreReadOnly",
	RespAccessDenied:          "AccessDenied",
	RespStoreNotAvailable:     "StoreNotAvailable",
	RespDeviceBusy:            "DeviceBusy",
	RespInvalidParentObject:   "InvalidParentObject",
	RespInvalidParameter:      "InvalidParameter",
	RespSessionAlreadyOpen:    "SessionAlreadyOpen",
	RespTransactionCancelled:  "TransactionCancelled",
	RespInvalidObjectPropCode: "InvalidObjectPropCode",
	RespObjectTooLarge:        "ObjectTooLarge",
}

func (r ResponseCode) String() string {
	if name, ok := respNames[r]; ok {
		return name
	}
	return fmt.Sprintf("response 0x%04x", uint16(r))
}

// EventCode is what an event container reports happened on the device.
type EventCode uint16

// The events a device sends.
const (
	EventCancelTransaction  EventCode = 0x4001
	EventObjectAdded        EventCode = 0x4002
	EventObjectRemoved      EventCode = 0x4003
	EventStoreAdded         EventCode = 0x4004
	EventStoreRemoved       EventCode = 0x4005
	EventObjectInfoChanged  EventCode = 0x4007
	EventDeviceInfoChanged  EventCode = 0x4008
	EventStorageInfoChanged EventCode = 0x400C
)

// ObjectFormat is the format code an object carries.
type ObjectFormat uint16

// The formats a Conn creates.
const (
	FormatUndefined   ObjectFormat = 0x3000
	FormatAssociation ObjectFormat = 0x3001
)

// AssociationGenericFolder is the association type of a folder.
const AssociationGenericFolder uint16 = 0x0001

// PropCode is an MTP object property.
type PropCode uint16

// The object properties a Conn reads and writes.
const (
	PropObjectSize     PropCode = 0xDC04
	PropObjectFileName PropCode = 0xDC07
)

// StorageID names one store on the device.
type StorageID uint32

// AllStorages asks for the objects of every store.
const AllStorages StorageID = 0xFFFFFFFF

// ObjectHandle names one object on the device for the session.
type ObjectHandle uint32

// Root is the parent of the objects at the top of a store.
const Root ObjectHandle = 0xFFFFFFFF
