package cloudtasksclient

import (
	"context"
	"encoding/json"
	"fmt"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"

	"github.com/GregMSThompson/finance-backend/internal/dto"
)

const alertDeliverPath = "/tasks/alert-deliver"

// Adapter enqueues tasks on a Cloud Tasks queue targeting the worker service.
type Adapter struct {
	client          *cloudtasks.Client
	queuePath       string
	workerURL       string
	serviceAccEmail string
}

// NewAdapter creates a Cloud Tasks adapter.
// serviceAccEmail is used for OIDC authentication on the enqueued HTTP tasks.
func NewAdapter(ctx context.Context, projectID, region, queueName, workerURL, serviceAccEmail string) (*Adapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create cloud tasks client: %w", err)
	}

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, region, queueName)

	return &Adapter{
		client:          client,
		queuePath:       queuePath,
		workerURL:       workerURL,
		serviceAccEmail: serviceAccEmail,
	}, nil
}

// Close releases the underlying Cloud Tasks client.
func (a *Adapter) Close() {
	a.client.Close()
}

// EnqueueAlertDelivery creates an HTTP task targeting the worker's alert delivery endpoint.
func (a *Adapter) EnqueueAlertDelivery(ctx context.Context, req dto.DeliverAlertRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal alert delivery request: %w", err)
	}

	task := &cloudtaskspb.Task{
		MessageType: &cloudtaskspb.Task_HttpRequest{
			HttpRequest: &cloudtaskspb.HttpRequest{
				HttpMethod: cloudtaskspb.HttpMethod_POST,
				Url:        a.workerURL + alertDeliverPath,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       body,
				AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
					OidcToken: &cloudtaskspb.OidcToken{
						ServiceAccountEmail: a.serviceAccEmail,
						Audience:            a.workerURL,
					},
				},
			},
		},
	}

	if _, err := a.client.CreateTask(ctx, &cloudtaskspb.CreateTaskRequest{
		Parent: a.queuePath,
		Task:   task,
	}); err != nil {
		return fmt.Errorf("create cloud task: %w", err)
	}

	return nil
}