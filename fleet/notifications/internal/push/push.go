// Package push sends messages to customers.
package push

import "context"

// Sender sends one message to one customer.
type Sender interface {
	Send(ctx context.Context, customerID, text string) error
}
