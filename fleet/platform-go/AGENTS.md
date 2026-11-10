# platform-go

Shared Go libraries used by every service. These rules apply fleet-wide.

## Conventions
- Money is `money.Pence` (an int64 count of pence). Never use float64 for amounts.
- Log with `fleet.example/platform-go/log`. Never use `fmt.Println` or the standard `log` package in services.
- Publish and subscribe only through `bus.Bus`, with topic constants from `fleet.example/proto/events/v1`.
- Format amounts for people with `money.Format`.

## Commands
- `go test ./...`
