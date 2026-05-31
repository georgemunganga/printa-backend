package comms

import (
"context"
"github.com/georgemunganga/printa-backend/internal/modules/notification"
)

// Dispatcher wraps the comms Service to implement notification.ChannelDispatcher.
// This adapter bridges the two interfaces without changing either.
type Dispatcher struct{ svc Service }

// NewDispatcher creates a notification.ChannelDispatcher backed by the comms service.
func NewDispatcher(svc Service) notification.ChannelDispatcher {
return &Dispatcher{svc: svc}
}

// Send implements notification.ChannelDispatcher.
func (d *Dispatcher) Send(ctx context.Context, event notification.Event) error {
return d.svc.SendEvent(ctx, event)
}
