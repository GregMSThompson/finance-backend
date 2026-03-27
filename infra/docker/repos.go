package docker

import (
	"fmt"
	"path"

	pulumidocker "github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/artifactregistry"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/GregMSThompson/finance-backend/infra/common"
)

type Manager struct {
	provider *gcp.Provider
}

func New(prov *gcp.Provider) *Manager {
	return &Manager{provider: prov}
}

func SetupArtifactRegistry(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "artifactRegistry", &projects.ServiceArgs{
		Service: pulumi.String("artifactregistry.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

func (m *Manager) CreateAPIRepo(ctx *pulumi.Context, res ...pulumi.Resource) (*artifactregistry.Repository, error) {
	return m.createRepo(ctx, "apiRepository", "api", "Docker repository for API images", res...)
}

func (m *Manager) CreateWorkerRepo(ctx *pulumi.Context, res ...pulumi.Resource) (*artifactregistry.Repository, error) {
	return m.createRepo(ctx, "workerRepository", "worker", "Docker repository for worker images", res...)
}

func (m *Manager) BuildImage(ctx *pulumi.Context, cmdName string, res ...pulumi.Resource) (*pulumidocker.Image, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require("region")

	cmdPath := path.Join("../cmd", cmdName)
	dockerfilePath := path.Join(cmdPath, "Dockerfile")

	internalHash, err := common.GenerateHash("../internal")
	if err != nil {
		return nil, err
	}
	pkgHash, err := common.GenerateHash("../pkg")
	if err != nil {
		return nil, err
	}
	cmdHash, err := common.GenerateHash(cmdPath)
	if err != nil {
		return nil, err
	}
	hash := common.AppendHash(internalHash, pkgHash, cmdHash)

	return pulumidocker.NewImage(ctx, "image"+common.CapitalizeFirst(cmdName), &pulumidocker.ImageArgs{
		Build: pulumidocker.DockerBuildArgs{
			Platform:   pulumi.String("linux/amd64"),
			Context:    pulumi.String(".."),
			Dockerfile: pulumi.String(dockerfilePath),
		},
		ImageName: pulumi.String(fmt.Sprintf("%s-docker.pkg.dev/%s/%s/finannce-%s:%s", region, projectID, cmdName, cmdName, hash)),
	},
		pulumi.DependsOn(res),
	)
}

func (m *Manager) createRepo(ctx *pulumi.Context, resourceName, repositoryID, description string, res ...pulumi.Resource) (*artifactregistry.Repository, error) {
	gcpCfg := config.New(ctx, "gcp")
	region := gcpCfg.Require("region")

	return artifactregistry.NewRepository(ctx, resourceName, &artifactregistry.RepositoryArgs{
		Format:       pulumi.String("DOCKER"),
		RepositoryId: pulumi.String(repositoryID),
		Location:     pulumi.String(region),
		Description:  pulumi.String(description),
	},
		pulumi.Provider(m.provider),
		pulumi.DependsOn(res),
	)
}
