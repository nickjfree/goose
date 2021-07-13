package wire


import (
	"bytes"
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucas-clemente/quic-go/quicvarint"
	"github.com/songgao/water/waterutil"
	"github.com/pkg/errors"
	"goose/pkg/tunnel"
)


const HTTP_BUFFERSIZE = 2048


var (
	serverIp = ""
	// http1 client
	client = http.Client{
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{Timeout: 15 * time.Second}
				conn, err := d.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				serverIp = strings.Split(conn.RemoteAddr().String(), ":")[0]
				logger.Printf("Remote IP: %s\n", serverIp)
				return conn, err
			},
		},
	}

	// http3 client
	client3 = http.Client{
		Transport: &http3.RoundTripper{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

type byteReader interface {
	io.ByteReader
	io.Reader
}

// byteReaderImpl - implementation of byteReader interface
type byteReaderImpl struct{ io.Reader }

func (br *byteReaderImpl) ReadByte() (byte, error) {
	b := make([]byte, 1)
	if _, err := br.Reader.Read(b); err != nil {
		return 0, err
	}
	return b[0], nil
}


// http wire
type HTTPWire struct {
	// base
	BaseWire
	// reader
	reader io.Reader
	// writer
	writer io.Writer
}


func NewHTTPWire(r io.Reader, w io.Writer) (Wire, error) {
	return &HTTPWire{
		BaseWire: BaseWire{},
		reader: r,
		writer: w,
	}, nil
}


// read message from tun
func (w *HTTPWire) Read() (tunnel.Message, error) {

	// read dataFrame <payload size><payload data>
	br := &byteReaderImpl{w.reader}
	// read payload size
	len, err := quicvarint.Read(br)
	if err != nil {
		return nil, errors.Wrap(err, "read http stream error, payload size")
	}
	if len > HTTP_BUFFERSIZE {
		return nil, errors.Errorf("client buffer size(%d) to big", len)
	}
	// read payload
	payload := make ([]byte, len)
	_, err = io.ReadFull(w.reader, payload)
	if err != nil {
		return nil, errors.Wrap(err, "read http stream")
	}
	srcIP := waterutil.IPv4Source(payload)
	dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// log the packet
	// logger.Printf("recv: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, n)
	return tunnel.NewTunMessage(dstIP.String(), srcIP.String(), payload), nil
}

// send message to tun
func (w *HTTPWire) Write(msg tunnel.Message) (error) {

	payload, ok := msg.Payload().([]byte)
	if !ok {
		logger.Printf("invalid payload format %+v", payload)
		return nil
	}
	buf := &bytes.Buffer{}
	// write payload size and content
	quicvarint.Write(buf, uint64(len(payload)))
	buf.Write(payload)
	// send http data <payload size><payload data>
	if _, err := w.writer.Write(buf.Bytes()); err != nil {
		return errors.Wrapf(err, "write http stream")
	}
	switch flusher := w.writer.(type) {
	case http.Flusher:
		flusher.Flush()
	case *bufio.Writer:
		if err := flusher.Flush(); err != nil {
			return errors.Wrapf(err, "write http stream(flush)")
		}
	default:
	}
	// srcIP := waterutil.IPv4Source(payload)
	// dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// // log the packet
	// logger.Printf("send: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, len(payload))
	return nil
}

// http server
type HTTPServer struct{
	// the tunnel
	tunnel *tunnel.Tunnel
	// is http1
	isHttp11 bool
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// switch protocols
	// w.WriteHeader(http.StatusSwitchingProtocols)
	logger.Printf("new connection %s", r.RemoteAddr)
	var reader io.Reader
	var writer io.Writer
	if s.isHttp11 {
		// switching protocol
		w.Header().Set("Upgrade", "goose")
		w.Header().Set("Connection", "Upgrade")
		w.WriteHeader(http.StatusSwitchingProtocols)
		w.(http.Flusher).Flush()
		// for http1 server, the r.body is now NoBody. we can't read from it. use a hijacker
		hj, ok := w.(http.Hijacker)
		if !ok {
			logger.Printf("response is not a hijacker %+v", w)
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			logger.Printf("hijack failed %+v", err)
			return
		}
		defer conn.Close()
		// set rw
		reader = bufrw.Reader
		writer = bufrw.Writer
	} else {
		// http3(quic), in order to use the same HTTPWire interface for both http3 and http11
		// we must use 200 ok here, or we will not be able to send a body, unless using datastream() which is ugly
		// quic-go(responseWriter)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		// set rw
		reader = r.Body
		writer = w
	}
	wire, err := NewHTTPWire(reader, writer)
	if err != nil {
		logger.Printf("create http wire error %+v", err)
		return
	}
	// handle client register
	logger.Printf("waiting for client %s to register addr", r.RemoteAddr)
	clientAddr, err := HandleRegisterAddr(wire, s.tunnel)
	if err != nil {
		logger.Printf("client register error: %+v", err)
		return
	}
	// add client to port
	port, err := s.tunnel.AddPort(clientAddr, false)
	if err != nil {
		logger.Printf("add port error %+v", err)
		return
	}
	logger.Printf("wire quit: %s", Communicate(wire, port))
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-goose"},
	}
}


// serve http1
func ServeHTTP(tunnel *tunnel.Tunnel) {
	server := &http.Server{
		Addr:      "0.0.0.0:443",
		Handler:   &HTTPServer{
			tunnel: tunnel,
			isHttp11: true,
		},
		TLSConfig: generateTLSConfig(),
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		logger.Fatalf("Error: %v", err)
	}
}


// serve http3
func ServeHTTP3(tunnel *tunnel.Tunnel) {
	server := http3.Server{
		Server: &http.Server{
			Addr:      "0.0.0.0:55556",
			Handler:   &HTTPServer{
				tunnel: tunnel,
				isHttp11: false,
			},
			TLSConfig: generateTLSConfig(),
		},
	}

	err := server.ListenAndServe()
	if err != nil {
		logger.Fatalf("Error: %v", err)
	}
}


// http client wire
type HTTPClientWire struct {
	BaseWire
	// http response
	reader io.ReadCloser
	// write
	writer io.Writer
}

func connectLoop(client *http.Client, method string, endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {
	pr, pw := io.Pipe()
	defer pr.Close()
	req, err := http.NewRequest(method, endpoint, ioutil.NopCloser(pr))
	// set TE to identify, so client won't use chunked, causing cloudflare blocking for request body
	req.TransferEncoding = []string{"identity"}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "goose")
	if err != nil {
		logger.Printf("create request error %+v", err)
	}
	// http client request
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "request http error")
	}
	logger.Printf("switching protocol successful, server(%s) return code: %d", req.RemoteAddr, resp.StatusCode)
	stream := resp.Body
	defer stream.Close()

	// create wire
	wire, err := NewHTTPWire(stream, pw)
	if err != nil {
		return errors.Wrap(err, "create http wire error")
	}
	// register to server
	tunnelGateway, err := RegisterAddr(wire, localAddr)
	if err != nil {
		logger.Printf("register to server error: %+v", err)
		return errors.Wrap(err, "")
	}
	port, err := tunnel.AddPort(tunnelGateway, true)
	if err != nil {
		return errors.Wrap(err, "add port error")
	}
	logger.Printf("add port %s", tunnelGateway)
	// setup route
	defer tunnel.RestoreRoute()
	tunnel.SetupRoute(tunnelGateway, serverIp)
	// handle stream
	return Communicate(wire, port)
}


// connect to remote http1 server
func ConnectHTTP(endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {

	for {
		logger.Printf("connecting to server %s", endpoint)
		logger.Printf("connection to server %s failed: %+v", endpoint, connectLoop(&client, "GET", endpoint, localAddr,tunnel))
		time.Sleep(time.Duration(5) * time.Second)
	}
}

// connect to remote http3 server
func ConnectHTTP3(endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {

	for {
		logger.Printf("connecting to server %s", endpoint)
		logger.Printf("connection to server %s failed: %+v", endpoint, connectLoop(&client3, "GET_0RTT", endpoint, localAddr,tunnel))
		time.Sleep(time.Duration(5) * time.Second)
	}
}
