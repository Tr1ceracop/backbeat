package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

type Request struct {
	Command string          `json:"command"`
	Args    json.RawMessage `json:"args,omitempty"`
}

type Response struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

type TrackArgs struct {
	IssueKey string `json:"issue_key"`
}

type StatusData struct {
	State        string `json:"state"`
	IssueKey     string `json:"issue_key,omitempty"`
	SessionStart string `json:"session_start,omitempty"`
	IsMeeting    bool   `json:"is_meeting"`
	TodayTotal   int    `json:"today_total_sec"`
	TodayMeeting int    `json:"today_meeting_sec"`
}

type SyncData struct {
	WorklogsCreated int `json:"worklogs_created"`
}

type Handler interface {
	HandleStatus() (*StatusData, error)
	HandleStop() error
	HandleTrack(args TrackArgs) error
	HandleSync() (*SyncData, error)
}

type Server struct {
	socketPath string
	handler    Handler
	listener   net.Listener
	logger     zerolog.Logger
}

func SocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("backbeat-%d", os.Getuid()))
		os.MkdirAll(dir, 0o700)
	}
	return filepath.Join(dir, "backbeat.sock")
}

func NewServer(handler Handler, logger zerolog.Logger) (*Server, error) {
	sockPath := SocketPath()

	// Remove stale socket
	if _, err := os.Stat(sockPath); err == nil {
		// Try connecting to check if another daemon is running
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			conn.Close()
			return nil, fmt.Errorf("another backbeat daemon is already running")
		}
		os.Remove(sockPath)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", sockPath, err)
	}

	return &Server{
		socketPath: sockPath,
		handler:    handler,
		listener:   listener,
		logger:     logger,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				s.logger.Error().Err(err).Msg("accept failed")
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		s.writeError(conn, "invalid request")
		return
	}

	var resp Response
	switch req.Command {
	case "status":
		data, err := s.handler.HandleStatus()
		if err != nil {
			resp = Response{Error: err.Error()}
		} else {
			raw, _ := json.Marshal(data)
			resp = Response{OK: true, Data: raw}
		}

	case "stop":
		if err := s.handler.HandleStop(); err != nil {
			resp = Response{Error: err.Error()}
		} else {
			resp = Response{OK: true}
		}

	case "track":
		var args TrackArgs
		if err := json.Unmarshal(req.Args, &args); err != nil {
			resp = Response{Error: "invalid track args"}
		} else if err := s.handler.HandleTrack(args); err != nil {
			resp = Response{Error: err.Error()}
		} else {
			resp = Response{OK: true}
		}

	case "sync":
		data, err := s.handler.HandleSync()
		if err != nil {
			resp = Response{Error: err.Error()}
		} else {
			raw, _ := json.Marshal(data)
			resp = Response{OK: true, Data: raw}
		}

	default:
		resp = Response{Error: fmt.Sprintf("unknown command: %s", req.Command)}
	}

	json.NewEncoder(conn).Encode(resp)
}

func (s *Server) writeError(conn net.Conn, msg string) {
	json.NewEncoder(conn).Encode(Response{Error: msg})
}

func (s *Server) Close() error {
	s.listener.Close()
	return os.Remove(s.socketPath)
}

// SendCommand sends a command to the daemon via IPC and returns the response.
func SendCommand(req Request) (*Response, error) {
	conn, err := net.Dial("unix", SocketPath())
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w (is backbeat running?)", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &resp, nil
}
