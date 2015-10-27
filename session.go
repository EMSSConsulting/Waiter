package waiter

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/consul/api"
)

// Session describes a customer session which will ensure that should the consuming
// application close, the client's wait entries are removed.
type Session struct {
	ID string

	client   *api.Client
	closedCh chan struct{}
	closeCh  <-chan struct{}
}

// NewSession creates a new Consul session with delete behaviour which will ensure
// that all entries created under it are removed when the owning application exits.
func NewSession(client *api.Client, name string) (*Session, error) {
	s := &Session{
		client:  client,
		closeCh: makeShutdownCh(),
	}

	session := client.Session()

	id, _, err := session.Create(&api.SessionEntry{
		Name:      name,
		Behavior:  "delete",
		LockDelay: time.Nanosecond,
		TTL:       "10s",
	}, nil)

	if err != nil {
		return nil, err
	}

	go func() {
		select {
		case <-s.closeCh:
			s.Close()
		}
	}()

	err = session.RenewPeriodic("10s", id, nil, s.closedCh)
	if err != nil {
		return nil, err
	}

	s.ID = id
	return s, nil
}

// Close will terminate an active session and should be used in conjunction with
// defer to ensure that the session is correctly terminated when your application
// exits.
func (s *Session) Close() error {
	if s == nil || s.ID == "" {
		return nil
	}

	session := s.client.Session()
	_, err := session.Destroy(s.ID, nil)

	if err != nil {
		return err
	}

	select {
	case s.closedCh <- struct{}{}:
	default:
	}
	s.ID = ""
	return nil
}

// makeShutdownCh creates a channel which will emit whenever a SIGTERM/SIGINT
// is received by the application - this is used to close any active sessions.
func makeShutdownCh() <-chan struct{} {
	resultCh := make(chan struct{})

	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			<-signalCh
			resultCh <- struct{}{}
		}
	}()

	return resultCh
}
