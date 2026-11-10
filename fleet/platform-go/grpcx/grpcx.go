// Package grpcx dials other services by name.
package grpcx

// Conn is a connection to a named service.
type Conn struct{ Target string }

// Dial returns a connection to the service, resolved through the mesh.
func Dial(service string) *Conn { return &Conn{Target: service + ".svc.fleet:443"} }
