package imagefamily

import (
	"github.com/zoom/karpenter-oci/pkg/apis/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

func FindCompatibleInstanceType(instanceTypes []*cloudprovider.InstanceType, imgs []v1alpha1.Image) map[string][]*cloudprovider.InstanceType {

	imgIDS := map[string][]*cloudprovider.InstanceType{}
	for _, instanceType := range instanceTypes {
		for _, ami := range imgs {

			amiRequirement := scheduling.NewNodeSelectorRequirements() // mock the image requirement since image does not support requirements yet
			if err := instanceType.Requirements.Compatible(
				amiRequirement,
				scheduling.AllowUndefinedWellKnownLabels,
			); err == nil {
				imgIDS[ami.Id] = append(imgIDS[ami.Id], instanceType)
				break
			}
		}
	}
	return imgIDS
}
