// connect to server through ipfs network
package wire

import (
	"bytes"
	"context"
	"io"
	"fmt"
	"encoding/json"
	// "strings"
	"time"
	"crypto/rand"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/lucas-clemente/quic-go/quicvarint"
	"github.com/songgao/water/waterutil"
	"github.com/pkg/errors"
	"goose/pkg/tunnel"
)

// ipfs bootstrap node
var( 
	bootstraps = []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}
)

// ipfs wire
type IPFSWire struct {
	// base
	BaseWire
	// reader
	reader io.Reader
	// writer
	writer io.Writer
}


func NewIPFSWire(r io.Reader, w io.Writer) (Wire, error) {
	return &IPFSWire{
		BaseWire: BaseWire{},
		reader: r,
		writer: w,
	}, nil
}

// read message from ipfs wire
func (w *IPFSWire) Read() (tunnel.Message, error) {

	// read dataFrame <payload size><payload data>
	br := &byteReaderImpl{w.reader}
	// read payload size
	len, err := quicvarint.Read(br)
	if err != nil {
		return nil, errors.Wrap(err, "read ipfs stream error, payload size")
	}
	if len > HTTP_BUFFERSIZE {
		return nil, errors.Errorf("client buffer size(%d) to big", len)
	}
	// read payload
	payload := make ([]byte, len)
	_, err = io.ReadFull(w.reader, payload)
	if err != nil {
		return nil, errors.Wrap(err, "read ipfs stream")
	}
	srcIP := waterutil.IPv4Source(payload)
	dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// log the packet
	// logger.Printf("recv: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, n)
	return tunnel.NewTunMessage(dstIP.String(), srcIP.String(), payload), nil
}

// send message to ipfs wire
func (w *IPFSWire) Write(msg tunnel.Message) (error) {

	payload, ok := msg.Payload().([]byte)
	if !ok {
		logger.Printf("invalid payload format %+v", payload)
		return nil
	}
	buf := &bytes.Buffer{}
	// write payload size and content
	quicvarint.Write(buf, uint64(len(payload)))
	buf.Write(payload)
	// send ipfs data <payload size><payload data>
	if _, err := w.writer.Write(buf.Bytes()); err != nil {
		return errors.Wrapf(err, "write ipfs stream")
	}
	// srcIP := waterutil.IPv4Source(payload)
	// dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// // log the packet
	// logger.Printf("send: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, len(payload))
	return nil
}

// p2p host
type P2PHost struct {
	// libp2p host
	host.Host
	// peer info channel for auto relay
	peerChan chan peer.AddrInfo
	// host context
	ctx context.Context
	// cancel
	cancel context.CancelFunc 
	// the tunnel
	tunnel *tunnel.Tunnel
}

func NewP2PHost(tunnel *tunnel.Tunnel) (*P2PHost, error) {
	// crreate peer chan
	peerChan := make(chan peer.AddrInfo, 100)
	// create p2p host
	host, err := createHost(peerChan)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &P2PHost{
		Host: host,
		ctx: ctx,
		peerChan: peerChan,
		cancel : cancel,
		tunnel: tunnel,
	}
	if h.Bootstrap(bootstraps); err != nil {
		return nil, err
	}
	return h, nil
}

// bootstrap with public peers
func (h *P2PHost) Bootstrap(peers []string) error {
	// bootstrap timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 300)
	defer cancel()

	if len(peers) < 1 {
		return errors.New("not enough bootstrap peers")
	}
	errs := make(chan error, len(peers))
	var wg sync.WaitGroup
	for _, str := range peers {	
		maddr := ma.StringCast(str)
		p, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			logger.Fatalln(err)
		}
		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()

			logger.Printf("%s bootstrapping to %s", h.ID(), p.ID)

			h.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
			if err := h.Connect(ctx, p); err != nil {
				logger.Printf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}
			logger.Printf("bootstrapped with %v", p.ID)
		}(*p)
	}
	wg.Wait()
	// our failure condition is when no connection attempt succeeded.
	// So drain the errs channel, counting the results.
	close(errs)
	count := 0
	var err error
	for err = range errs {
		if err != nil {
			count++
		}
	}
	if count == len(peers) {
		return fmt.Errorf("failed to bootstrap. %s", err)
	}
	logger.Printf("bootstrap with %d node", len(peers)-count)
	return nil
}

// run some background jobs
func (h *P2PHost) Background() error {
	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	for {
		select {
		case <- h.ctx.Done():
			return nil
		case <- ticker.C:
			// show network state
			peers := h.Network().Peers()
			peerList := []peer.AddrInfo{}
			for _, peer := range peers {
				peerList = append(peerList, h.Peerstore().PeerInfo(peer))
			}
			// find relays
			for _, peer := range peerList {
				select {
				case h.peerChan <- peer:
				case <- h.ctx.Done():
					return nil
				}
			}
			logger.Printf("feeding %d peers to RelayFinder", len(peerList))
			addrText, err := json.MarshalIndent(h.Addrs(), "", "  ")
			if err != nil {
				logger.Printf("error %s", errors.WithStack(err))
			}
			logger.Printf("peerid: %s\nrelays: %s\n",  h.ID(), addrText)
		}
	}
}

// handle client stream
func (h *P2PHost) HandleClientStream(s network.Stream, localAddr string) error {
	defer s.Close()	
	// create wire
	wire, err := NewIPFSWire(s, s)
	if err != nil {
		return errors.Wrap(err, "create ipfs wire error")
	}
	// register to server
	tunnelGateway, err := RegisterAddr(wire, localAddr)
	if err != nil {
		logger.Printf("register to server error: %+v", err)
		return errors.Wrap(err, "")
	}
	port, err := h.tunnel.AddPort(tunnelGateway, true)
	if err != nil {
		return errors.Wrap(err, "add port error")
	}
	logger.Printf("add port %s", tunnelGateway)
	// get the relay's addr
	peerAddr := s.Conn().RemoteMultiaddr()
	logger.Printf("remote peer addr is %s", peerAddr)
	serverIp, err = peerAddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return errors.Wrap(err, "failed to get relays ip")
	}
	defer h.tunnel.RestoreRoute()
	h.tunnel.SetupRoute(tunnelGateway, serverIp)
	// handle stream
	return Communicate(wire, port)
}

// handle server stream
func (h *P2PHost) HandleServerStream(s network.Stream) error {
	wire, err := NewIPFSWire(s, s)
	if err != nil {
		logger.Printf("create ipfs wire error %+v", err)
		return err
	}
	// handle client register
	logger.Printf("waiting for client %s to register addr", s.ID())
	clientAddr, err := HandleRegisterAddr(wire, h.tunnel)
	if err != nil {
		logger.Printf("client register error: %+v", err)
		return err 
	}
	// add client to port
	port, err := h.tunnel.AddPort(clientAddr, false)
	if err != nil {
		logger.Printf("add port error %+v", err)
		return err
	}
	// communicate
	err = Communicate(wire, port)
	logger.Printf("wire quit: %s", err)
	return errors.WithStack(err)
}

// create libp2p node
// circuit relay need to be enabled to hide the real server ip.
func createHost(peerChan <- chan peer.AddrInfo) (host.Host, error) {

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var idht *dht.IpfsDHT
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/udp/4001/quic"),
		libp2p.Identity(priv),
		// enable relay
		libp2p.EnableRelay(),
		// enable node to use relay for wire communication
		libp2p.EnableAutoRelay(autorelay.WithPeerSource(peerChan)),
		// force node belive it is behind a NAT firewall to force using relays
		libp2p.ForceReachabilityPrivate(),
		// hole punching
		libp2p.EnableHolePunching(),

		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		// enable routing
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err = dht.New(context.Background(), h)
			return idht, err
		}),
	}
	host, err := libp2p.New(opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return host, nil
}

// run as a goose server in ipfs network
// well.. the server is not actually a ipfs host, instead it talks in goose/0.0.1 protocol
func ServeIPFS(tunnel *tunnel.Tunnel) {
	host, err := NewP2PHost(tunnel)
	if err != nil {
		logger.Fatalf("Error: %+v", err)
	}
	// set server stream hanlder
	host.SetStreamHandler("/goose/0.0.1", func(s network.Stream) {
		defer s.Close()
		logger.Printf("received new stream %s", s.ID())
		if err := host.HandleServerStream(s); err != nil {
			logger.Printf("handle stream error %+v", errors.WithStack(err))
		}
	})
	// go backgroud relay refresh jobs
	if err := host.Background(); err != nil {
		logger.Fatalf("Error: %v", err)
	}
}

// connect to remote peer once
func connectLoopIPFS(host *P2PHost, endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {

	// decode peerid
	peerID, err := peer.Decode(endpoint)
	if err != nil {
		return errors.WithStack(err)
	}
	p := peer.AddrInfo{
		ID: peerID,
	}
	// connect to the peer
	ctx, cancel := context.WithCancel(context.Background())
	// ctx = network.WithUseTransient(ctx, "")
	defer cancel()
	s, err := host.NewStream(ctx, p.ID, "/goose/0.0.1")
	if err != nil {
		return errors.WithStack(err)
	}
	if err := host.HandleClientStream(s, localAddr); err != nil {
		return err
	}
	return nil
}

// connect to remote peer by PeerId
func ConnectIPFS(endpoint string, localAddr string, tunnel *tunnel.Tunnel) error {

	host, err := NewP2PHost(tunnel)
	if err != nil {
		logger.Fatalf("Error: %v", err)
	}
	for {
		logger.Printf("connecting to server %s", endpoint)
		logger.Printf("connection to server %s failed: %+v", endpoint, connectLoopIPFS(host, endpoint, localAddr, tunnel))
		time.Sleep(time.Second * 5)
	}
	return nil
}
