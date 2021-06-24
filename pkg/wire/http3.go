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


const HTTP3_BUFFERSIZE = 2048


var (
	// http3 client
	client = http.Client{
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

// http3 quic wire
type HTTP3Wire struct {
	// base
	BaseWire
	// reader
	reader io.Reader
	// writer
	writer io.Writer		
}


func NewHTTP3Wire(r io.Reader, w io.Writer) (Wire, error) {
	return &HTTP3Wire{
		BaseWire: BaseWire{},
		reader: r,
		writer: w,
	}, nil
}


// read message from tun
func (w *HTTP3Wire) Read() (tunnel.Message, error) {

	// read dataFrame <payload size><payload data>
	br := &byteReaderImpl{w.reader}	
	// read payload size
	len, err := quicvarint.Read(br)
	if err != nil {
		return nil, errors.Wrap(err, "read http3 stream error, payload size")
	}
	if len > HTTP3_BUFFERSIZE {
		return nil, errors.Errorf("client buffer size(%d) to big", len)
	}
	// read payload
	payload := make ([]byte, len)
	_, err = io.ReadFull(w.reader, payload)
	if err != nil {
		return nil, errors.Wrap(err, "read http3 stream")
	}
	srcIP := waterutil.IPv4Source(payload)
	dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// log the packet
	// logger.Printf("recv: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, n)
	return tunnel.NewTunMessage(dstIP.String(), srcIP.String(), payload), nil
}

// send message to tun
func (w *HTTP3Wire) Write(msg tunnel.Message) (error) {

	payload, ok := msg.Payload().([]byte)
	if !ok {
		logger.Printf("invalid payload format %+v", payload)
		return nil
	}
	buf := &bytes.Buffer{}
	// write payload size and content
	quicvarint.Write(buf, uint64(len(payload)))
	buf.Write(payload)
	// send http3 data <payload size><payload data>
	if _, err := w.writer.Write(buf.Bytes()); err != nil {
		return errors.Wrapf(err, "write http3 stream")
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


// http3 server
type HTTP3Server struct{
	// the tunnel
	tunnel *tunnel.Tunnel
}


func (s *HTTP3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// switch protocols
	// w.WriteHeader(http.StatusSwitchingProtocols)
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
	// create a HTTP3 wire
	wire, err := NewHTTP3Wire(r.Body, w)
	if err != nil {
		logger.Printf("create http3 wire error %+v", err)
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


// serve http3
func ServeHTTP3(tunnel *tunnel.Tunnel) {
	server := http3.Server{
		Server: &http.Server{
			Addr:      "0.0.0.0:55556",
			Handler:   &HTTP3Server{
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



// http3 client
type HTTP3ClientWire struct {
	BaseWire
	// http response
	reader io.ReadCloser
	// write
	writer io.Writer
}


//
func NewHTTP3ClientWire (r io.ReadCloser, w io.Writer)  (Wire, error) {
	return &HTTP3ClientWire{
		reader: r,
		writer: w,
	}, nil
}


func (w *HTTP3ClientWire) Write(msg tunnel.Message) (error) {
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
		return errors.Wrap(err, "error write http3 stream")
	}
	// srcIP := waterutil.IPv4Source(payload)
	// dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// // log the packet
	// logger.Printf("send: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, len(payload))
	return nil
}

func (w *HTTP3ClientWire) Read() (tunnel.Message, error) {

	payload := make ([]byte, HTTP3_BUFFERSIZE)

	br := &byteReaderImpl{w.reader}
	len, err := quicvarint.Read(br)
	if err != nil {
		return nil, errors.Wrap(err, "read http3 stream error")
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


func connectLoop(endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {
	pr, pw := io.Pipe()
	defer pr.Close()
	req, err := http.NewRequest(http.MethodGet, endpoint, ioutil.NopCloser(pr))
	if err != nil {
		logger.Printf("create request error %+v", err)
	}
	// http3 client request
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "request http3 error")
	}
	logger.Printf("switching protocol successful, server return code: %d", resp.StatusCode)
	stream := resp.Body
	defer stream.Close()

	// create wire
	wire, err := NewHTTP3ClientWire(stream, pw)
	if err != nil {
		return errors.Wrap(err, "create http3 wire error")
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

// connect to remote server
func ConnectHTTP3(endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {

	for {
		logger.Printf("connecting to server %s", endpoint)
		logger.Printf("connection to server %s failed: %+v", endpoint, connectLoop(endpoint, localAddr,tunnel))
		time.Sleep(10 * time.Second)
	}
}
