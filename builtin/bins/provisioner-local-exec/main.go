package main

import (
	"github.com/r3labs/terraform/builtin/provisioners/local-exec"
	"github.com/r3labs/terraform/plugin"
	"github.com/r3labs/terraform/terraform"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProvisionerFunc: func() terraform.ResourceProvisioner {
			return new(localexec.ResourceProvisioner)
		},
	})
}
