package websocket

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type Conn struct {
	rw    ioReadWriter
	conn  net.Conn
	state ws.State
	mu    sync.Mutex
}

type ioReadWriter interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (n int, err error) {
	if c.reader != nil {
		return c.reader.Read(p)
	}
	return c.Conn.Read(p)
}

func Wrap(conn net.Conn, state ws.State) *Conn {
	return &Conn{
		rw:    conn,
		conn:  conn,
		state: state,
	}
}

func UpgradeHTTP(r *http.Request, w http.ResponseWriter) (*Conn, error) {
	conn, rw, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return nil, err
	}
	return &Conn{
		rw:    &bufferedConn{Conn: conn, reader: rw.Reader},
		conn:  conn,
		state: ws.StateServerSide,
	}, nil
}

func Dial(ctx context.Context, url string) (*Conn, error) {
	conn, reader, _, err := ws.Dial(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Conn{
		rw:    &bufferedConn{Conn: conn, reader: reader},
		conn:  conn,
		state: ws.StateClientSide,
	}, nil
}

func (c *Conn) RawConn() net.Conn {
	return c.conn
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) ReadData() ([]byte, ws.OpCode, error) {
	if c == nil {
		return nil, 0, fmt.Errorf("websocket conn is nil")
	}
	if c.state == ws.StateServerSide {
		return wsutil.ReadClientData(c.rw)
	}
	return wsutil.ReadServerData(c.rw)
}

func (c *Conn) ReadText() ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("websocket conn is nil")
	}
	if c.state == ws.StateServerSide {
		return wsutil.ReadClientText(c.rw)
	}
	return wsutil.ReadServerText(c.rw)
}

func (c *Conn) ReadBinary() ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("websocket conn is nil")
	}
	if c.state == ws.StateServerSide {
		return wsutil.ReadClientBinary(c.rw)
	}
	return wsutil.ReadServerBinary(c.rw)
}

func (c *Conn) WriteMessage(op ws.OpCode, payload []byte) error {
	if c == nil {
		return fmt.Errorf("websocket conn is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == ws.StateServerSide {
		return wsutil.WriteServerMessage(c.rw, op, payload)
	}
	return wsutil.WriteClientMessage(c.rw, op, payload)
}

func (c *Conn) WriteText(payload []byte) error {
	return c.WriteMessage(ws.OpText, payload)
}

func (c *Conn) WriteBinary(payload []byte) error {
	return c.WriteMessage(ws.OpBinary, payload)
}

func (c *Conn) Ping(payload []byte) error {
	return c.WriteMessage(ws.OpPing, payload)
}

func (c *Conn) Pong(payload []byte) error {
	return c.WriteMessage(ws.OpPong, payload)
}

func (c *Conn) CloseWithStatus(code ws.StatusCode, reason string) error {
	return c.WriteMessage(ws.OpClose, ws.NewCloseFrameBody(code, reason))
}
