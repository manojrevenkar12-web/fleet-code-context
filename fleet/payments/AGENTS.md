# payments

Authorises, captures and refunds card payments.

## Layout
- `internal/server/` implements PaymentService.
- `internal/consumer/` refunds on OrderCancelled.

## Commands
- `make test`
- `go test ./internal/server/...`

## Notes
- Refunds must be idempotent per order.
