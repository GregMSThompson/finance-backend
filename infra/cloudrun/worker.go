package cloudrun

import "github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"

type Worker struct {
	provider *gcp.Provider
}

// NewWorker creates a worker Cloud Run deployer bound to a provider.
func NewWorker(prov *gcp.Provider) *Worker {
	return &Worker{provider: prov}
}
