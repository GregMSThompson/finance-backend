// seed-notification writes a test Notification to Firestore so the worker's
// /tasks/notification-deliver endpoint can be called directly from Postman using
// the test API key.
//
// Usage:
//
//	go run ./cmd/tools/seed-notification --project=<gcp-project> --uid=<user-id>
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"

	"github.com/GregMSThompson/finance-backend/internal/models"
)

func main() {
	project := flag.String("project", "", "GCP project ID (required)")
	uid := flag.String("uid", "", "User ID to seed the notification for (required)")
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

	notification := &models.Notification{
		NotificationID: uuid.NewString(),
		Source:         models.NotificationSourceAlert,
		SourceID:       "test-alert-id",
		Title:          "Large transaction detected",
		Body:           "A transaction of $500.00 was detected on your account.",
		Delivery:       models.DeliveryPush,
	}

	_, err = client.Collection("users").Doc(*uid).Collection("notifications").Doc(notification.NotificationID).Set(ctx, notification)
	if err != nil {
		log.Fatalf("write notification: %v", err)
	}

	fmt.Printf("notification written\n")
	fmt.Printf("  notificationId: %s\n", notification.NotificationID)
	fmt.Printf("  userId:         %s\n", *uid)
	fmt.Printf("\nPostman body:\n")
	fmt.Printf(`{"notificationId": "%s", "userId": "%s", "delivery": "push"}`+"\n", notification.NotificationID, *uid)
}
