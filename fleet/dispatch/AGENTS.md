# dispatch

Matches placed orders to riders and publishes RiderAssigned.

## Layout
- `internal/assign/` picks a rider.
- `internal/eta/` estimates arrival.
- `internal/consumer/` listens for OrderPlaced.

## Commands
- `make test`
- `go test ./internal/eta/...`

## Notes
- ETAs are in minutes.
- Candidate riders come from `ridersv1.RiderServiceClient.ListAvailableRiders`.
