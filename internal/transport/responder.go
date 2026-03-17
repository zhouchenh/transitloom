package transport

import (
	"fmt"
	"net"
)

// ProbeResponder listens on a UDP port and answers Transitloom probe requests.
//
// Its role is to enable external reachability verification: when a remote
// node or coordinator probes this node's external endpoint, the ProbeResponder
// echoes the nonce back so the prober can confirm that the path is reachable
// end-to-end.
//
// A ProbeResponder can run as a standalone listener or share a port with
// application data by integrating HandleProbeDatagram into an existing
// listener loop.
type ProbeResponder struct {
	conn *net.UDPConn
}

// NewProbeResponder creates a ProbeResponder listening on the given UDP address.
//
// Use "addr:0" to let the OS assign a free port. Retrieve the actual address
// with Addr() after creation.
func NewProbeResponder(listenAddr string) (*ProbeResponder, error) {
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("probe responder: resolve %q: %w", listenAddr, err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("probe responder: listen on %q: %w", listenAddr, err)
	}
	return &ProbeResponder{conn: conn}, nil
}

// Addr returns the local address the responder is listening on.
func (r *ProbeResponder) Addr() net.Addr {
	return r.conn.LocalAddr()
}

// Close stops the responder by closing the underlying connection.
// Serve returns after Close is called.
func (r *ProbeResponder) Close() error {
	return r.conn.Close()
}

// Serve runs the probe responder loop.
//
// It reads incoming UDP datagrams and responds to valid probe requests by
// echoing the nonce back with ProbeTypeResponse. Non-probe datagrams are
// silently discarded.
//
// Serve blocks until Close is called or the connection fails. It returns
// the error that caused the loop to stop (typically indicating the connection
// was closed).
func (r *ProbeResponder) Serve() error {
	buf := make([]byte, 64)
	for {
		n, remoteAddr, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		if !IsProbeDatagram(buf[:n]) {
			continue
		}
		_, nonce, parseErr := parseProbeDatagram(buf[:n])
		if parseErr != nil {
			continue
		}
		resp := encodeProbeResponse(nonce)
		// Best-effort write: if the write fails, the prober will time out and
		// retry on the next probe run. Do not abort the responder loop on a
		// write error to a single peer.
		_, _ = r.conn.WriteToUDP(resp[:], remoteAddr)
	}
}

// HandleProbeDatagram checks whether buf is a probe request and, if so,
// sends a probe response to remoteAddr using conn.
//
// Returns true if buf was a probe request and a response was attempted.
// Returns false if buf is not a probe datagram (caller should handle normally).
//
// This function is for integrating probe handling into an existing UDP listener
// that must multiplex probe datagrams and application traffic on the same port.
// The caller reads a datagram, then calls HandleProbeDatagram before processing
// it as application data.
func HandleProbeDatagram(conn *net.UDPConn, buf []byte, remoteAddr *net.UDPAddr) bool {
	if !IsProbeDatagram(buf) {
		return false
	}
	_, nonce, err := parseProbeDatagram(buf)
	if err != nil {
		return false
	}
	resp := encodeProbeResponse(nonce)
	_, _ = conn.WriteToUDP(resp[:], remoteAddr)
	return true
}
