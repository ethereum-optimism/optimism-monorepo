package flags

import (
	"fmt"
	"slices"
)

type DataAvailabilityType string

const (
	// data availability types
	CalldataType DataAvailabilityType = "calldata"
	BlobsType    DataAvailabilityType = "blobs"
	AutoType     DataAvailabilityType = "auto"
)

var DataAvailabilityTypes = []DataAvailabilityType{
	CalldataType,
	BlobsType,
	AutoType,
}

func (kind DataAvailabilityType) String() string {
	return string(kind)
}

func (kind *DataAvailabilityType) Set(value string) error {
	if !ValidDataAvailabilityType(DataAvailabilityType(value)) {
		return fmt.Errorf("unknown data-availability type: %q", value)
	}
	*kind = DataAvailabilityType(value)
	return nil
}

func (kind *DataAvailabilityType) Clone() any {
	cpy := *kind
	return &cpy
}

func ValidDataAvailabilityType(value DataAvailabilityType) bool {
	return slices.Contains(DataAvailabilityTypes, value)
}
