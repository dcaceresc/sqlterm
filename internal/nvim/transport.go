// Package nvim administra un editor Neovim aislado y su interfaz de grilla.
package nvim

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neovim/go-client/msgpack/rpc"
	client "github.com/neovim/go-client/nvim"
)

var (
	ErrInputPending    = errors.New("nvim: waiting for another key")
	ErrClosed          = errors.New("nvim: session closed")
	ErrExited          = errors.New("nvim: child exited")
	ErrTimeout         = errors.New("nvim: operation timed out")
	ErrUnsupportedMode = errors.New("nvim: operation is not supported in the current mode")
)

// Options configura el ejecutable directo, directorio de trabajo y grilla.
// Timeout limita cada operación; por defecto es de tres segundos.
type Options struct {
	Binary, Dir   string
	Width, Height int
	Timeout       time.Duration
}

// Session serializa operaciones RPC y conserva la última grilla publicada.
// Las notificaciones nunca esperan a un consumidor ni hacen RPC.
type Session struct {
	v          *client.Nvim
	cmd        *exec.Cmd
	timeout    time.Duration
	buffer     client.Buffer
	gate       chan struct{}
	done       chan struct{}
	changed    chan struct{}
	frameReady chan struct{}
	stopping   chan struct{}
	once       sync.Once
	reason     atomic.Pointer[error]
	frame      atomic.Pointer[Frame]
}

// Start inicia el hijo y espera el primer flush de la grilla principal.
func Start(o Options) (*Session, error) {
	if !validDimensions(o.Width, o.Height) {
		return nil, fmt.Errorf("nvim: invalid grid dimensions: %dx%d", o.Width, o.Height)
	}
	if o.Timeout < 0 {
		return nil, errors.New("nvim: negative timeout")
	}
	if o.Timeout == 0 {
		o.Timeout = 3 * time.Second
	}
	if o.Binary == "" {
		o.Binary = "nvim"
	}
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	defer null.Close()
	cmd := exec.Command(o.Binary, "--embed", "--clean", "-i", "NONE", "-n", "--cmd", "set noexrc noundofile noswapfile nobackup nowritebackup shada= virtualedit=onemore")
	cmd.Dir = o.Dir
	cmd.Stderr = null
	for _, v := range cmd.Environ() {
		key, _, _ := strings.Cut(v, "=")
		switch strings.ToUpper(key) {
		case "NVIM", "NVIM_LISTEN_ADDRESS", "NVIM_LOG_FILE", "VIMINIT", "EXINIT", "MYVIMRC", "MYGVIMRC", "VIMRUNTIME", "VIM":
			continue
		}
		cmd.Env = append(cmd.Env, v)
	}
	cmd.Env = append(cmd.Env, "NVIM_LOG_FILE="+os.DevNull)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, fmt.Errorf("nvim: start %q: %w", o.Binary, err)
	}
	s := &Session{cmd: cmd, timeout: o.Timeout, gate: make(chan struct{}, 1), done: make(chan struct{}), stopping: make(chan struct{}), changed: make(chan struct{}, 1), frameReady: make(chan struct{}, 1)}
	v, err := client.New(out, in, in, func(string, ...any) {})
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	s.v = v
	var grid gridState
	registerErr := v.RegisterHandler("redraw", func(events ...[]any) { grid.apply(s.publish, events...) })
	go func() {
		serveErr := v.Serve()
		// Si el canal muere antes que el hijo, no se queda esperando a Wait.
		if serveErr != nil {
			s.stop(fmt.Errorf("%w: %v", ErrExited, serveErr))
		}
		// Un EOF también puede provenir de un hijo que cerró stdout sin salir.
		_ = cmd.Process.Kill()
		waitErr := cmd.Wait()
		if waitErr != nil {
			s.stop(fmt.Errorf("%w: %v", ErrExited, waitErr))
		} else {
			s.stop(ErrExited)
		}
		close(s.done)
	}()
	if registerErr != nil {
		_ = s.Close()
		return nil, registerErr
	}
	err = s.run(func() error {
		var err error
		s.buffer, err = v.CurrentBuffer()
		if err != nil {
			return err
		}
		if err = v.AttachUI(o.Width, o.Height, map[string]any{"ext_linegrid": true, "rgb": true}); err != nil {
			return err
		}
		return s.awaitFrame(o.Width, o.Height, 0)
	})
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("nvim: attach: %w", err)
	}
	return s, nil
}

func pulse(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
func (s *Session) publish(f Frame) { s.frame.Store(&f); pulse(s.frameReady); pulse(s.changed) }

// Frame devuelve el último flush; puede ser posterior o anterior a un Snapshot consultado por RPC.
func (s *Session) Frame() Frame {
	if f := s.frame.Load(); f != nil {
		return *f
	}
	return Frame{}
}

// Changed señala cambios de grilla. Los avisos se agrupan; no se cierra el canal.
func (s *Session) Changed() <-chan struct{} { return s.changed }

// Done se cierra cuando el hijo ha sido recogido. Err explica la terminación.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err informa la causa de terminación, o nil mientras la sesión siga disponible.
func (s *Session) Err() error {
	if err := s.reason.Load(); err != nil {
		return *err
	}
	return nil
}

func (s *Session) stop(reason error) {
	s.once.Do(func() { s.reason.Store(&reason); close(s.stopping); _ = s.cmd.Process.Kill() })
}

// Close termina y recoge el hijo, sin guardar datos de la sesión. Es idempotente.
func (s *Session) Close() error {
	s.stop(ErrClosed)
	select {
	case <-s.done:
		return nil
	case <-time.After(s.timeout):
		return fmt.Errorf("%w: waiting for child cleanup", ErrTimeout)
	}
}

func (s *Session) run(work func() error) error {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case <-s.stopping:
		return s.Err()
	case <-timer.C:
		return ErrTimeout
	case s.gate <- struct{}{}:
	}
	if err := s.Err(); err != nil {
		<-s.gate
		return err
	}
	result := make(chan error, 1)
	go func() { defer func() { <-s.gate }(); result <- work() }()
	select {
	case err := <-result:
		if errors.Is(err, rpc.ErrClosed) {
			s.stop(ErrExited)
		}
		if reason := s.Err(); reason != nil {
			return reason
		}
		return err
	case <-s.stopping:
		return s.Err()
	case <-timer.C:
		s.stop(ErrTimeout)
		return s.Err()
	}
}

func (s *Session) awaitFrame(w, h int, after uint64) error {
	for {
		if f := s.Frame(); f.Sequence > after && f.Width == w && f.Height == h {
			return nil
		}
		select {
		case <-s.stopping:
			return s.Err()
		case <-s.frameReady:
		}
	}
}

// Resize cambia las dimensiones y espera un flush de la grilla resultante.
func (s *Session) Resize(w, h int) error {
	if !validDimensions(w, h) {
		return fmt.Errorf("nvim: invalid grid dimensions: %dx%d", w, h)
	}
	return s.run(func() error {
		before := s.Frame()
		if before.Width == w && before.Height == h {
			return nil
		}
		if err := s.v.TryResizeUI(w, h); err != nil {
			return err
		}
		return s.awaitFrame(w, h, before.Sequence)
	})
}
