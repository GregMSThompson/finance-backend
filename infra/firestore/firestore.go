package firestore

import (
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/firestore"
	"github.com/pulumi/pulumi-gcp/sdk/v9/go/gcp/projects"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func SetupFirestore(ctx *pulumi.Context, prov *gcp.Provider) error {
	svc, err := enableFireStore(ctx, prov)
	if err != nil {
		return err
	}

	db, err := createDatabase(ctx, prov, svc)
	if err != nil {
		return err
	}

	if err := setupIndexes(ctx, prov, db, svc); err != nil {
		return err
	}

	return nil
}

func enableFireStore(ctx *pulumi.Context, prov *gcp.Provider) (*projects.Service, error) {
	return projects.NewService(ctx, "firestore", &projects.ServiceArgs{
		Service: pulumi.String("firestore.googleapis.com"),
	},
		pulumi.Provider(prov),
	)
}

func createDatabase(ctx *pulumi.Context, prov *gcp.Provider, res ...pulumi.Resource) (*firestore.Database, error) {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")
	region := gcpCfg.Require(("region"))

	db, err := firestore.NewDatabase(ctx, "firestoreDatabase", &firestore.DatabaseArgs{
		Name:       pulumi.String("(default)"),
		Project:    pulumi.String(projectID),
		LocationId: pulumi.String(region),
		Type:       pulumi.String("FIRESTORE_NATIVE"),
	},
		pulumi.Provider(prov),
		pulumi.DependsOn(res),
	)
	return db, err
}

func setupIndexes(ctx *pulumi.Context, prov *gcp.Provider, db *firestore.Database, res ...pulumi.Resource) error {
	if err := setupTransactionIndexes(ctx, prov, db, res...); err != nil {
		return err
	}

	if err := setupGoalSnapshotIndexes(ctx, prov, db, res...); err != nil {
		return err
	}

	return nil
}

// setupGoalSnapshotIndexes provisions the composite index the goal evaluator and
// progress endpoints rely on. A single (goalId ==, createdAt) index serves all
// three snapshot queries — Latest, ListForGoal, and ListForGoalSince (the
// createdAt range is on the same field as the order).
func setupGoalSnapshotIndexes(ctx *pulumi.Context, prov *gcp.Provider, db *firestore.Database, res ...pulumi.Resource) error {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")

	_, err := firestore.NewIndex(ctx, "goalSnapshotGoalIdCreatedAtDesc", &firestore.IndexArgs{
		Project:    pulumi.String(projectID),
		Database:   db.Name,
		Collection: pulumi.String("goalSnapshots"),
		QueryScope: pulumi.String("COLLECTION"),
		Fields:     indexFieldsWithNameOrder("DESCENDING", "goalId", "ASCENDING", "createdAt", "DESCENDING"),
	},
		pulumi.Provider(prov),
		pulumi.DependsOn(res),
	)
	return err
}

func setupTransactionIndexes(ctx *pulumi.Context, prov *gcp.Provider, db *firestore.Database, res ...pulumi.Resource) error {
	gcpCfg := config.New(ctx, "gcp")
	projectID := gcpCfg.Require("project")

	indexes := []struct {
		name   string
		fields firestore.IndexFieldArray
	}{
		{name: "txPendingDateAsc", fields: indexFields("pending", "ASCENDING", "date", "ASCENDING")},
		{name: "txPendingDateDesc", fields: indexFields("pending", "ASCENDING", "date", "DESCENDING")},
		{name: "txPendingDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "date", "DESCENDING")},
		{name: "txPfcPrimaryDateAsc", fields: indexFields("pfcPrimary", "ASCENDING", "date", "ASCENDING")},
		{name: "txPfcPrimaryDateDesc", fields: indexFields("pfcPrimary", "ASCENDING", "date", "DESCENDING")},
		{name: "txPfcPrimaryDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pfcPrimary", "ASCENDING", "date", "DESCENDING")},
		{name: "txAccountIdDateAsc", fields: indexFields("accountId", "ASCENDING", "date", "ASCENDING")},
		{name: "txAccountIdDateDesc", fields: indexFields("accountId", "ASCENDING", "date", "DESCENDING")},
		{name: "txAccountIdDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "accountId", "ASCENDING", "date", "DESCENDING")},
		{name: "txPendingPfcPrimaryDateAsc", fields: indexFields("pending", "ASCENDING", "pfcPrimary", "ASCENDING", "date", "ASCENDING")},
		{name: "txPendingPfcPrimaryDateDesc", fields: indexFields("pending", "ASCENDING", "pfcPrimary", "ASCENDING", "date", "DESCENDING")},
		{name: "txPendingPfcPrimaryDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "pfcPrimary", "ASCENDING", "date", "DESCENDING")},
		{name: "txPendingAccountIdDateAsc", fields: indexFields("pending", "ASCENDING", "accountId", "ASCENDING", "date", "ASCENDING")},
		{name: "txPendingAccountIdDateDesc", fields: indexFields("pending", "ASCENDING", "accountId", "ASCENDING", "date", "DESCENDING")},
		{name: "txPendingAccountIdDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "accountId", "ASCENDING", "date", "DESCENDING")},
		{name: "txPendingPfcPrimaryAccountIdDateAsc", fields: indexFields("pending", "ASCENDING", "pfcPrimary", "ASCENDING", "accountId", "ASCENDING", "date", "ASCENDING")},
		{name: "txPendingPfcPrimaryAccountIdDateDesc", fields: indexFields("pending", "ASCENDING", "pfcPrimary", "ASCENDING", "accountId", "ASCENDING", "date", "DESCENDING")},
		{name: "txPendingPfcPrimaryAccountIdDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "pfcPrimary", "ASCENDING", "accountId", "ASCENDING", "date", "DESCENDING")},
		// largestTransactions query support:
		// pending == false, optional accountId/pfcPrimary filters, date range on date, order by amount desc
		{name: "txPendingAmountDescDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "amount", "DESCENDING", "date", "DESCENDING")},
		{name: "txPendingAccountIdAmountDescDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "accountId", "ASCENDING", "amount", "DESCENDING", "date", "DESCENDING")},
		{name: "txPendingPfcPrimaryAmountDescDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "pfcPrimary", "ASCENDING", "amount", "DESCENDING", "date", "DESCENDING")},
		{name: "txPendingPfcPrimaryAccountIdAmountDescDateDescNameDesc", fields: indexFieldsWithNameOrder("DESCENDING", "pending", "ASCENDING", "pfcPrimary", "ASCENDING", "accountId", "ASCENDING", "amount", "DESCENDING", "date", "DESCENDING")},
	}

	for _, idx := range indexes {
		_, err := firestore.NewIndex(ctx, idx.name, &firestore.IndexArgs{
			Project:    pulumi.String(projectID),
			Database:   db.Name,
			Collection: pulumi.String("transactions"),
			QueryScope: pulumi.String("COLLECTION"),
			Fields:     idx.fields,
		},
			pulumi.Provider(prov),
			pulumi.DependsOn(res),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func indexFields(pathA, orderA, pathB, orderB string, rest ...string) firestore.IndexFieldArray {
	return indexFieldsWithNameOrder("ASCENDING", pathA, orderA, pathB, orderB, rest...)
}

func indexFieldsWithNameOrder(nameOrder string, pathA, orderA, pathB, orderB string, rest ...string) firestore.IndexFieldArray {
	fields := firestore.IndexFieldArray{
		&firestore.IndexFieldArgs{
			FieldPath: pulumi.String(pathA),
			Order:     pulumi.String(orderA),
		},
		&firestore.IndexFieldArgs{
			FieldPath: pulumi.String(pathB),
			Order:     pulumi.String(orderB),
		},
	}
	for i := 0; i+1 < len(rest); i += 2 {
		fields = append(fields, &firestore.IndexFieldArgs{
			FieldPath: pulumi.String(rest[i]),
			Order:     pulumi.String(rest[i+1]),
		})
	}
	fields = append(fields, &firestore.IndexFieldArgs{
		FieldPath: pulumi.String("__name__"),
		Order:     pulumi.String(nameOrder),
	})
	return fields
}
