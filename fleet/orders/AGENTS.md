# orders

Owns the order lifecycle: place, cancel, track.

## Layout
- `internal/handlers/` holds the gRPC handlers.
- `internal/store/` is the order store.

## Commands
- `make test`
- `make lint` before every push.

## Notes
- Prices come from pricing via `pricingclient.Quote`.
- Order totals are float64 pounds in the store.
