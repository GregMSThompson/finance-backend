module finance-worker

go 1.24.0

require (
	github.com/GregMSThompson/finance-backend/infra v0.0.0
	github.com/pulumi/pulumi/sdk/v3 v3.210.0
)

replace github.com/GregMSThompson/finance-backend/infra => ..
