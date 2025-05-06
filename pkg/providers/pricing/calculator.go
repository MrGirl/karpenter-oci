package pricing

import (
	"github.com/oracle/oci-go-sdk/v65/core"
	"math"
	"strings"
)

func Calculate(shape core.Shape, catalog PriceCatalog) float32 {

	items := catalog.FindPriceItems(*shape.Shape)
	priceLen := len(items)
	if priceLen == 0 { // not found, so do not recommend
		return math.MaxFloat32
	}

	if priceLen == 1 {

		it := items[0]
		if it.IsFree() {
			return 0
		}
		switch it.MetricName {
		case GpuPerHour:
			return float32(*shape.Gpus) * it.PricePerUnit()
		case OcpuPerHour:
			return *shape.Ocpus * it.PricePerUnit()
		case GigabytePerHour:
			return *shape.MemoryInGBs * it.PricePerUnit()
		case NodePerHour:
			if it.IsGpu() {
				return float32(*shape.Gpus) * it.PricePerUnit()
			} else {
				return *shape.Ocpus * it.PricePerUnit()
			}
		case NVMeTerabytePerHour:
			return *shape.LocalDisksTotalSizeInGBs * it.PricePerUnit()
		}
	} else if priceLen > 1 {

		var price float32 = 0
		for _, item := range items {

			if item.IsOcpuType() {
				price += *shape.Ocpus * item.PricePerUnit()
			} else if item.IsMemoryType() {
				price += *shape.MemoryInGBs * item.PricePerUnit()
			} else if item.IsNVMeType() {
				price += *shape.LocalDisksTotalSizeInGBs / 1024 * item.PricePerUnit()
			} else if item.IsMonthCommit() {
				continue
			} else if item.IsYearCommit() {
				continue
			} else if item.Is3YearCommit() {
				continue
			} else if item.IsHourlyCommit() {
				price += *shape.Ocpus * item.PricePerUnit() / float32(item.GetCpuNum())
				break
			} else {
				price += *shape.Ocpus * item.PricePerUnit()
			}
		}

		return price
	}

	return 0
}

func ContainOcpu(shape string) bool {
	return strings.Contains(shape, "OCPU")
}
func ContainMemory(shape string) bool {
	return strings.Contains(shape, "Memory")
}
func ContainNVMe(shape string) bool {
	return strings.Contains(shape, "NVMe")
}
