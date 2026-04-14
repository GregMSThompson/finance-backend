// seed-alert-event writes a test AlertEvent to Firestore so the worker's
// /tasks/alert-deliver endpoint can be called directly from Postman using
// the test API key.
//
// Usage:
//
//	go run ./cmd/tools/seed-alert-event --project=<gcp-project> --uid=<user-id>
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

func main() {
	project := flag.String("project", "", "GCP project ID (required)")
	uid := flag.String("uid", "", "User ID to seed the alert event for (required)")
	flag.Parse()

	if *project == "" || *uid == "" {
		flag.Usage()
		log.Fatal("--project and --uid are required")
	}

	ctx := context.Background()

	client, err := firestore.NewClient(ctx, *project)
	if err != nil {
		log.Fatalf("firestore client: %v", err)
	}
	defer client.Close()

	event := &models.AlertEvent{
		EventID:     uuid.NewString(),
		AlertID:     "test-alert-id",
		UserID:      *uid,
		Type:        models.AlertTypeLargeTransaction,
		Delivery:    models.DeliveryPush,
		Title:       "Large transaction detected",
		Body:        "A transaction of $500.00 was detected on your account.",
		TriggeredAt: time.Now(),
	}

	_, err = client.Collection("users").Doc(*uid).Collection("alert_events").Doc(event.EventID).Set(ctx, event)
	if err != nil {
		log.Fatalf("write alert event: %v", err)
	}

	fmt.Printf("alert event written\n")
	fmt.Printf("  eventId: %s\n", event.EventID)
	fmt.Printf("  userId:  %s\n", *uid)
	fmt.Printf("\nPostman body:\n")
	fmt.Printf(`{"alertEventId": "%s", "userId": "%s", "delivery": "push"}`+"\n", event.EventID, *uid)
}