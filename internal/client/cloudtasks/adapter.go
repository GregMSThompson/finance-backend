package cloudtasksclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

const notificationDeliverPath = "/tasks/notification-deliver"

// Adapter enqueues tasks on a Cloud Tasks queue targeting the worker service.
type Adapter struct {
	client          *cloudtasks.Client
	audience        string
	queuePath       string
	workerURL       string
	serviceAccEmail string
}

// NewAdapter creates a Cloud Tasks adapter.
// serviceAccEmail is used for OIDC authentication on the enqueued HTTP tasks.
func NewAdapter(ctx context.Context, projectID, region, queueName, audience, workerURL, serviceAccEmail string) (*Adapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create cloud tasks client: %w", err)
	}

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, region, queueName)

	return &Adapter{
		client:          client,
		audience:        audience,
		queuePath:       queuePath,
		workerURL:       workerURL,
		serviceAccEmail: serviceAccEmail,
	}, nil
}

// Close releases the underlying Cloud Tasks client.
func (a *Adapter) Close() {
	a.client.Close()
}

// EnqueueNotificationDelivery creates an HTTP task targeting the worker's
// notification delivery endpoint.
func (a *Adapter) EnqueueNotificationDelivery(ctx context.Context, req dto.DeliverNotificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal notification delivery request: %w", err)
	}
	return a.createTask(ctx, notificationDeliverPath, body)
}

// EnqueueJob creates an HTTP task targeting the worker route for the given job type.
// The route is derived from the job type by replacing "." with "/" and prefixing "/tasks/".
// e.g. JobType "plaid.sync" → POST /tasks/plaid/sync
func (a *Adapter) EnqueueJob(ctx context.Context, jobType models.JobType, uid, jobID string) error {
	if !jobType.IsValid() {
		return fmt.Errorf("unknown job type %q", jobType)
	}
	body, err := json.Marshal(dto.JobTaskRequest{UID: uid, JobID: jobID})
	if err != nil {
		return fmt.Errorf("marshal job task request: %w", err)
	}
	path := "/tasks/" + strings.ReplaceAll(string(jobType), ".", "/")
	return a.createTask(ctx, path, body)
}

func (a *Adapter) createTask(ctx context.Context, path string, body []byte) error {
	task := &cloudtaskspb.Task{
		MessageType: &cloudtaskspb.Task_HttpRequest{
			HttpRequest: &cloudtaskspb.HttpRequest{
				HttpMethod: cloudtaskspb.HttpMethod_POST,
				Url:        a.workerURL + path,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       body,
				AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
					OidcToken: &cloudtaskspb.OidcToken{
						ServiceAccountEmail: a.serviceAccEmail,
						Audience:            a.audience,
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
