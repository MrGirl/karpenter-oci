package pricing

import (
	"testing"
	"time"
)

func TestPriceListSyncer_Start(t *testing.T) {

	endpint := "https://apexapps.oracle.com/pls/apex/cetools/api/v1/products/"

	var period int64 = 60 * 2

	syncer := NewPriceListSyncer(endpint, period)
	syncer.Start()

	time.Sleep(20 * time.Second)

	shapes := []string{
		"BM.DenseIO.E4.128",
		"BM.DenseIO.E5.128",
		"BM.DenseIO2.52",
		"BM.GPU.A10.4",
		"BM.GPU.B4.8",
		"BM.GPU2.2",
		"BM.GPU3.8",
		"BM.GPU4.8",
		"BM.HPC2.36",
		"BM.Optimized3.36",
		"BM.Standard.A1.160",
		"BM.Standard.B1.44",
		"BM.Standard.E2.64",
		"BM.Standard.E3.128",
		"BM.Standard.E4.128",
		"BM.Standard.E5.192",
		"BM.Standard.E6.256",
		"BM.Standard1.36",
		"BM.Standard2.52",
		"BM.Standard3.64",
		"VM.DenseIO.E4.Flex",
		"VM.DenseIO.E5.Flex",
		"VM.DenseIO2.16",
		"VM.DenseIO2.24",
		"VM.DenseIO2.8",
		"VM.GPU.A10.1",
		"VM.GPU.A10.2",
		"VM.GPU2.1",
		"VM.GPU3.1",
		"VM.Optimized3.Flex",
		"VM.Standard.A1.Flex",
		"VM.Standard.A2.Flex",
		"VM.Standard.B1.1",
		"VM.Standard.B1.16",
		"VM.Standard.B1.2",
		"VM.Standard.B1.4",
		"VM.Standard.B1.8",
		"VM.Standard.E2.1",
		"VM.Standard.E2.1.Micro",
		"VM.Standard.E2.2",
		"VM.Standard.E2.4",
		"VM.Standard.E2.8",
		"VM.Standard.E3.Flex",
		"VM.Standard.E4.Flex",
		"VM.Standard.E5.Flex",
		"VM.Standard.E6.Flex",
		"VM.Standard1.1",
		"VM.Standard1.16",
		"VM.Standard1.2",
		"VM.Standard1.4",
		"VM.Standard1.8",
		"VM.Standard2.1",
		"VM.Standard2.16",
		"VM.Standard2.2",
		"VM.Standard2.24",
		"VM.Standard2.4",
		"VM.Standard2.8",
		"VM.Standard3.Flex",
	}

	for _, shape := range shapes {

		items := syncer.PriceCatalog.FindPriceItems(shape)
		if len(items) > 0 {

			for _, item := range items {
				t.Logf("%s: %+v", shape, item)
			}
		} else {
			t.Errorf("%s: not found", shape)
		}
	}

	//shape := "BM.GPU.A10.4"
	//shape := "BM.GPU.B4.8" //deprecated
	//shape := "BM.GPU2.2"
	//shape := "BM.Optimized3.36" //need to test again
	//shape := "BM.Standard.A1.160" // which is in service category virtual machine
	//shape := "BM.Standard.B1.44"
	//shape := "BM.Standard.E2.64" // which is in service category virtual machine
	//shape := "BM.Standard1.36" // deprecated
	//shape := "BM.Standard3.64" // which is in service category VMware
	//shape := "VM.DenseIO.E5.Flex" // return multiple, e5 ocpu, e5 memory, e5 nvme, should be e5nvme
	//shape := "VM.GPU.A10.1"
	//shape := "VM.Standard.A2.Flex" // multiple, A2 ocpu, A2 memory
	//shape := "VM.Standard.B1.1"
	//shape := "BM.DenseIO2.52"
	//shape := "BM.GPU2.2"
	//shape := "BM.HPC2.36" // need to test again
	//shape := "BM.Optimized3.36"

	//items := syncer.PriceCatalog.FindPriceItems(shape)
	//if len(items) > 0 {
	//	for _, item := range items {
	//
	//		t.Logf("%v: %+v", shape, item)
	//	}
	//}

}
