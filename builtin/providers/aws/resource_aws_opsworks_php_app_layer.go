package aws

import (
	"github.com/r3labs/terraform/helper/schema"
)

func resourceAwsOpsworksPhpAppLayer() *schema.Resource {
	layerType := &opsworksLayerType{
		TypeName:         "php-app",
		DefaultLayerName: "PHP App Server",

		Attributes: map[string]*opsworksLayerTypeAttribute{},
	}

	return layerType.SchemaResource()
}
