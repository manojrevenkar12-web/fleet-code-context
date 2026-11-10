# proto

Contracts for every service in the fleet. `.proto` files are the source of truth; the `.pb.go` files next to
them are generated.

- Regenerate with `make generate` after editing a `.proto` file.
- Never renumber or reuse a field number. Add new fields at the end.
- Events live in `events/v1`. Every event has a topic constant named `Topic<Message>`.
- Breaking changes need a new package version (`v2`), not an edit in place.
