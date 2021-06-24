package wire


import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucas-clemente/quic-go/quicvarint"
	"github.com/songgao/water/waterutil"
	"github.com/pkg/errors"
	"goose/pkg/tunnel"
)


const HTTP_BUFFERSIZE = 2048


var (
	// http1 client
	client = http.Client{
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
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
	flusher, ok := w.writer.(http.Flusher)
	if !ok {
		return errors.Errorf("not a flusher %+v", w.writer)
	}
	flusher.Flush()
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
}


func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// switch protocols
	// w.WriteHeader(http.StatusSwitchingProtocols)
	logger.Printf("new connection %s", r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
	// create a HTTP3 wire
	wire, err := NewHTTPWire(r.Body, w)
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


// http client wire
func NewHTTPClientWire (r io.ReadCloser, w io.Writer)  (Wire, error) {
	return &HTTPClientWire{
		reader: r,
		writer: w,
	}, nil
}

// write data to wire
func (w *HTTPClientWire) Write(msg tunnel.Message) (error) {
	payload, ok := msg.Payload().([]byte)
	if !ok {
		logger.Printf("msg it not valid %+v", msg)
		return nil
	}
	buf := &bytes.Buffer{}
	quicvarint.Write(buf, uint64(len(payload)))
	buf.Write(payload)
	// the writer guarantees one dataframe will be send
	if _, err := w.writer.Write(buf.Bytes()); err != nil {
		return errors.Wrap(err, "error write http stream")
	}
	// srcIP := waterutil.IPv4Source(payload)
	// dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// // log the packet
	// logger.Printf("send: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, len(payload))
	return nil
}

// read data from wire
func (w *HTTPClientWire) Read() (tunnel.Message, error) {

	payload := make ([]byte, HTTP_BUFFERSIZE)

	br := &byteReaderImpl{w.reader}
	len, err := quicvarint.Read(br)
	if err != nil {
		return nil, errors.Wrap(err, "read http stream error")
	}
	// read the payload
	n, err := io.ReadFull(w.reader, payload[:len])
	if err != nil {
		return nil, err
	}
	srcIP := waterutil.IPv4Source(payload)
	dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// log the packet
	// logger.Printf("recv: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, n)
	return tunnel.NewTunMessage(dstIP.String(), srcIP.String(), payload[:n]), nil
}


func connectLoop(client *http.Client, endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {
	pr, pw := io.Pipe()
	defer pr.Close()
	req, err := http.NewRequest(http.MethodGet, endpoint, ioutil.NopCloser(pr))
	if err != nil {
		logger.Printf("create request error %+v", err)
	}
	// http client request
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "request http error")
	}
	logger.Printf("switching protocol successful, server return code: %d", resp.StatusCode)
	stream := resp.Body
	defer stream.Close()

	// create wire
	wire, err := NewHTTPClientWire(stream, pw)
	if err != nil {
		return errors.Wrap(err, "create http wire error")
	}
	// register to server
	serverAddr, err := RegisterAddr(wire, localAddr)
	if err != nil {
		logger.Printf("register to server error: %+v", err)
		return errors.Wrap(err, "")
	}
	port, err := tunnel.AddPort(serverAddr, true)
	if err != nil {
		return errors.Wrap(err, "add port error")
	}
	logger.Printf("add port %s", serverAddr)
	// handle stream
	return Communicate(wire, port)
}


// connect to remote http1 server
func ConnectHTTP(endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {

	for {
		logger.Printf("connecting to server %s", endpoint)
		logger.Printf("connection to server %s failed: %+v", endpoint, connectLoop(&client, endpoint, localAddr,tunnel))
		time.Sleep(10 * time.Second)
	}
}

// connect to remote http3 server
func ConnectHTTP3(endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {

	for {
		logger.Printf("connecting to server %s", endpoint)
		logger.Printf("connection to server %s failed: %+v", endpoint, connectLoop(&client3, endpoint, localAddr,tunnel))
		time.Sleep(10 * time.Second)
	}
}
