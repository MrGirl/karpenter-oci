package internalmodel

import "github.com/oracle/oci-go-sdk/v65/core"

type WrapShape struct {
	core.Shape
	CalcCpu               int64
	CalMemInGBs           int64
	AvailableDomains      []string
	CalMaxVnic            int64
	CalMaxBandwidthInGbps int64
}
