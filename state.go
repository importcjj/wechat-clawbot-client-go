package clawbot

// ClientState represents the lifecycle state of a Client.
type ClientState int

const (
	StateNew            ClientState = iota // Just created
	StateLoggingIn                         // Login() in progress
	StateReady                             // Has credentials, can Start
	StateRunning                           // Start() running, long-polling
	StateSessionExpired                    // Server returned errcode -14
	StateStopped                           // Start() ended
)

func (s ClientState) String() string {
	switch s {
	case StateNew:
		return "new"
	case StateLoggingIn:
		return "logging_in"
	case StateReady:
		return "ready"
	case StateRunning:
		return "running"
	case StateSessionExpired:
		return "session_expired"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}
