package event

import (
	"log/slog"

	"github.com/nats-io/nats.go"
)

// natsConnAdapter wraps a *nats.Conn so it satisfies the event.NATSConn
// interface. Story 15.2/22.2.
type natsConnAdapter struct {
	conn *nats.Conn
}

// NewNATSConnFromURL dials the given NATS URL and returns a NATSConn ready to
// plug into NATSBus. Reconnect behaviour is delegated to nats.go's built-in
// handler so we don't re-implement backoff here.
func NewNATSConnFromURL(url string) (NATSConn, error) {
	c, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(200*1e6), // 200ms
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("nats reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, err
	}
	return &natsConnAdapter{conn: c}, nil
}

func (a *natsConnAdapter) Publish(subject string, data []byte) error {
	return a.conn.Publish(subject, data)
}

func (a *natsConnAdapter) Subscribe(subject string, handler func(subject string, data []byte)) (Unsubscribe, error) {
	sub, err := a.conn.Subscribe(subject, func(m *nats.Msg) {
		handler(m.Subject, m.Data)
	})
	if err != nil {
		return nil, err
	}
	return natsUnsub{sub: sub}, nil
}

func (a *natsConnAdapter) Close() {
	if a.conn != nil {
		a.conn.Close()
	}
}

// Status reports the NATS connection status — satisfies NATSConnStatus so
// `hive status` can surface whether the cluster link is live (Story 15.3).
func (a *natsConnAdapter) Status() string {
	if a.conn == nil {
		return "closed"
	}
	switch a.conn.Status() {
	case nats.CONNECTED:
		return "connected"
	case nats.RECONNECTING:
		return "reconnecting"
	case nats.CLOSED:
		return "closed"
	case nats.CONNECTING:
		return "connecting"
	case nats.DRAINING_SUBS, nats.DRAINING_PUBS:
		return "draining"
	default:
		return "unknown"
	}
}

type natsUnsub struct {
	sub *nats.Subscription
}

func (u natsUnsub) Unsubscribe() error {
	if u.sub != nil {
		return u.sub.Unsubscribe()
	}
	return nil
}
