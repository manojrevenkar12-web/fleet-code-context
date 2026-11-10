module fleet.example/notifications

go 1.24

require (
	fleet.example/platform-go v0.0.0
	fleet.example/proto v0.0.0
)

replace (
	fleet.example/platform-go => ../platform-go
	fleet.example/proto => ../proto
)
