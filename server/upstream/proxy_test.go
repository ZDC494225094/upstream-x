package upstream

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// startTestSocks5 launches a minimal SOCKS5 server (CONNECT only,
// optional username/password auth) and returns its URL. connections
// counts accepted SOCKS5 sessions so tests can prove traffic really
// went through the proxy.
func startTestSocks5(t *testing.T, user, pass string) (string, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	connections := &atomic.Int32{}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go handleSocks5(t, conn, user, pass)
		}
	}()

	auth := ""
	if user != "" {
		auth = url.UserPassword(user, pass).String() + "@"
	}
	return fmt.Sprintf("socks5://%s%s", auth, listener.Addr().String()), connections
}

func handleSocks5(t *testing.T, conn net.Conn, wantUser, wantPass string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 512)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return
	}
	method := byte(0x00)
	if wantUser != "" {
		method = 0x02
	}
	if _, err := conn.Write([]byte{0x05, method}); err != nil {
		return
	}
	if method == 0x02 {
		if _, err := io.ReadFull(conn, buf[:2]); err != nil {
			return
		}
		ulen := int(buf[1])
		if _, err := io.ReadFull(conn, buf[:ulen+1]); err != nil {
			return
		}
		gotUser := string(buf[:ulen])
		plen := int(buf[ulen])
		if _, err := io.ReadFull(conn, buf[:plen]); err != nil {
			return
		}
		gotPass := string(buf[:plen])
		if gotUser != wantUser || gotPass != wantPass {
			_, _ = conn.Write([]byte{0x01, 0x01})
			return
		}
		if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
			return
		}
	}
	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return
	}
	if buf[0] != 0x05 || buf[1] != 0x01 { // CONNECT only
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	var host string
	switch buf[3] {
	case 0x01:
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
	case 0x03:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		l := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:l]); err != nil {
			return
		}
		host = string(buf[:l])
	default:
		_, _ = conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(buf[:2])
	target, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))), 5*time.Second)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()
	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	go func() { _, _ = io.Copy(target, conn) }()
	_, _ = io.Copy(conn, target)
}

func TestValidateProxyURL(t *testing.T) {
	valid := []string{"", "socks5://127.0.0.1:1080", "socks5h://user:pass@host:1080", "http://127.0.0.1:8080", "https://proxy.example.com:8443"}
	for _, raw := range valid {
		if err := ValidateProxyURL(raw); err != nil {
			t.Errorf("expected %q valid, got %v", raw, err)
		}
	}
	invalid := []string{"ftp://127.0.0.1:21", "socks5://", "://bad", "127.0.0.1:1080"}
	for _, raw := range invalid {
		if err := ValidateProxyURL(raw); err == nil {
			t.Errorf("expected %q invalid", raw)
		}
	}
}

func TestClientRoutesThroughSocks5(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":"203.0.113.7"}`))
	}))
	defer target.Close()

	proxyURL, connections := startTestSocks5(t, "alice", "s3cret")
	client := NewClientWithProxyProvider(func() (string, error) { return proxyURL, nil })

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.do(req)
	if err != nil {
		t.Fatalf("request through socks5: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "203.0.113.7") {
		t.Fatalf("unexpected body: %s", body)
	}
	if connections.Load() == 0 {
		t.Fatal("socks5 server saw no connection; traffic bypassed the proxy")
	}

	// Wrong proxy password must fail.
	badClient := NewClientWithProxyProvider(func() (string, error) {
		return strings.Replace(proxyURL, "s3cret", "wrong", 1), nil
	})
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, target.URL, nil)
	if _, err := badClient.do(req2); err == nil {
		t.Fatal("expected auth failure with wrong proxy password")
	}
}
