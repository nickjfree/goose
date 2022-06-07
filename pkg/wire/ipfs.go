// connect to server through ipfs network
package wire

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"fmt"
	"encoding/json"
	"strings"
	"time"
	"crypto/rand"
	"path/filepath"
	"sync"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	dis_routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
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

const (
	// advertise server prefix
	PrefixGooseServer = "/goose/0.1.0/server"
	// advertise network prefix
	PrefixGooseNetwork = "/goose/0.1.0/network"
	// advertise relay prefix
	PrefixGooseRelay = "/goose/0.1.0/relay"
	// connection protection tag
	connectionTag = "goose"
	// protocol name
	protocolName = "/goose/0.1.0"
	// transiend error string
	transientErrorString = "transient connection"
	// key size
	keyBits = 2048
)


var (
	// manager
	ipfsWireManager *IPFSWireManager
)


// register ipfs wire manager
func init() {
	ipfsWireManager = NewIPFSWireManager()
	RegisterWireManager(ipfsWireManager)
}

// ipfs wire
type IPFSWire struct {
	// base
	BaseWire
	// stream
	s network.Stream
	// close func
	closeFunc func () error
}


// read message from ipfs wire
func (w *IPFSWire) Read() (tunnel.Message, error) {

	// read dataFrame <payload size><payload data>
	br := &byteReaderImpl{w.s}
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
	_, err = io.ReadFull(w.s, payload)
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
	if _, err := w.s.Write(buf.Bytes()); err != nil {
		return errors.Wrapf(err, "write ipfs stream")
	}
	// srcIP := waterutil.IPv4Source(payload)
	// dstIP := waterutil.IPv4Destination(payload)
	// proto := waterutil.IPv4Protocol(payload)
	// // log the packet
	// logger.Printf("send: src %s, dst %s, protocol %+v, len %d", srcIP, dstIP, proto, len(payload))
	return nil
}

// send message to ipfs wire
func (w *IPFSWire) Close() (error) {
	return w.closeFunc()
}


// IPFS wire manager
type IPFSWireManager struct {
	BaseWireManager
	*P2PHost
}

func NewIPFSWireManager() *IPFSWireManager {
	host, err := NewP2PHost()
	if err != nil {
		logger.Fatalf("Error: %+v", err)
	}
	// set server stream hanlder
	host.SetStreamHandler(protocolName, func(s network.Stream) {
		defer s.Close()
		host.ConnManager().Protect(s.Conn().RemotePeer(), connectionTag)
		logger.Printf("received new stream %s", s.ID())
		// close func
		close := func () error {
			// unprotect connecttion
			host.ConnManager().Unprotect(s.Conn().RemotePeer(), connectionTag)
			// close stream
			s.Close()
			return nil
		}
		// got an inbound wire
		inboundWires <- &IPFSWire{
			s: s,
			closeFunc: close,
		}
	})
	// disable advertise for debug
	// host.SetAdvertise(false, namespace)
	// do backgroud relay refresh jobs
	go host.Background()
	return &IPFSWireManager{
		P2PHost: host,
	}
}

func (m *IPFSWireManager) Connect(endpoint string) error {

	peerID, err := peer.Decode(endpoint)
	if err != nil {
		return errors.WithStack(err)
	}
	p := &peer.AddrInfo{
		ID: peerID,
	}
	// connect to the peer
	ctx, cancel := context.WithCancel(context.Background())
	retries := 30
	for {
		s, err := m.NewStream(ctx, p.ID, protocolName)
		msg := fmt.Sprintf("%+v", err)
		if err != nil && retries > 0 && strings.Contains(msg, transientErrorString) {
			logger.Printf("transient connection, try again for %s", p.ID)
			time.Sleep(time.Second * 15)
			retries -= 1
			continue
		} else if err != nil {
			return errors.WithStack(err)
		}
		m.ConnManager().Protect(s.Conn().RemotePeer(), connectionTag)
		// close func
		close := func () error {
			// unprotect connecttion
			m.ConnManager().Unprotect(s.Conn().RemotePeer(), connectionTag)
			// close stream
			s.Close()
			// cancel stream context
			cancel()
			return nil
		}
		// got an outbound wire
		inboundWires <- &IPFSWire{
			s: s,
			closeFunc: close,
		}
		return nil
	}
	return nil
}

func (m *IPFSWireManager) Protocol() string {
	return "ipfs"
}

// p2p host
type P2PHost struct {
	// libp2p host
	host.Host
	// discovery
	*dis_routing.RoutingDiscovery
	// peer info channel for auto relay
	peerChan chan peer.AddrInfo
	// host context
	ctx context.Context
	// cancel
	cancel context.CancelFunc 
	// the tunnel
	tunnel *tunnel.Tunnel
	// advertise
	advertise bool
	// advertise namespace
	namespace string
}

func NewP2PHost() (*P2PHost, error) {
	// crreate peer chan
	peerChan := make(chan peer.AddrInfo, 100)
	// create p2p host	
	host, dht, err := createHost(peerChan)
	if err != nil {
		return nil, err
	}
	routingDiscovery := dis_routing.NewRoutingDiscovery(dht)
	ctx, cancel := context.WithCancel(context.Background())
	h := &P2PHost{
		Host: host,
		RoutingDiscovery: routingDiscovery,
		ctx: ctx,
		peerChan: peerChan,
		cancel : cancel,
		advertise: false,
	}
	if h.Bootstrap(bootstraps); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *P2PHost) SetAdvertise(state bool, namespace string) {
	h.advertise = state
	h.namespace = namespace
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


func (h *P2PHost) GetAdvertiseName() string {
	return filepath.ToSlash(filepath.Join(PrefixGooseServer, h.namespace))
}

// run some background jobs
func (h *P2PHost) Background() error {

	// state ticker
	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	// advertise ticker
	advertiseTicker := time.NewTicker(time.Hour * 6)
	defer advertiseTicker.Stop()
	// bootstrap ticker
	bootstrapTicker := time.NewTicker(time.Second * 900)
	defer bootstrapTicker.Stop()

	

	if h.advertise {
		h.Advertise(h.ctx, h.GetAdvertiseName())
	}
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
			logger.Printf("%d peers", len(peerList))
			addrText, err := json.MarshalIndent(h.Addrs(), "", "  ")
			if err != nil {
				logger.Printf("error %s", errors.WithStack(err))
			}
			logger.Printf("peerid: %s\naddrs: %s\n", h.ID(), addrText)
		case <- advertiseTicker.C:
			// time to advertise
			if h.advertise {
				h.Advertise(h.ctx, h.GetAdvertiseName())
			}
		case <- bootstrapTicker.C:
			// bootstrap refesh
			if err := h.Bootstrap(bootstraps); err != nil {
				logger.Printf("bootstrap error %+v", err)
			}
		}
	}
}

// // handle client stream
// func (h *P2PHost) HandleClientStream(s network.Stream, localAddr string) error {
// 	// protecte the peer from connection manager trimming
// 	h.ConnManager().Protect(s.Conn().RemotePeer(), connectionTag)
// 	defer h.ConnManager().Unprotect(s.Conn().RemotePeer(), connectionTag)
// 	// create wire
// 	_, err := NewIPFSWire(s, s)
// 	if err != nil {
// 		return errors.Wrap(err, "create ipfs wire error")
// 	}
// 	return nil
// }

// // handle server stream
// func (h *P2PHost) HandleServerStream(s network.Stream) error {
// 	// protecte the peer from connection manager trimming
// 	h.ConnManager().Protect(s.Conn().RemotePeer(), connectionTag)
// 	defer h.ConnManager().Unprotect(s.Conn().RemotePeer(), connectionTag)

// 	_, err := NewIPFSWire(s, s)
// 	if err != nil {
// 		logger.Printf("create ipfs wire error %+v", err)
// 		return err
// 	}
// 	return nil
// }

// get privkey, save it to local path
func getPrivKey(path string) (crypto.PrivKey, error) {
	if _, err := os.Stat(path); err != nil {
		// file not exists, create a new one
		keyFile, err := os.Create(path)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer keyFile.Close()
		priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, keyBits, rand.Reader)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		raw, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if _, err := keyFile.Write(raw); err != nil {
			return nil, errors.WithStack(err)
		}
	}
	// open key file
	keyFile, err := os.Open(path)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer keyFile.Close()
	if data, err := ioutil.ReadAll(keyFile); err != nil {
		return nil, errors.WithStack(err)
	} else {
		privKey, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return privKey, nil
	}
}


// create libp2p node
// circuit relay need to be enabled to hide the real server ip.
func createHost(peerChan <- chan peer.AddrInfo) (host.Host, *dht.IpfsDHT, error) {

	if err := os.MkdirAll("data", 0644); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	priv, err := getPrivKey("data/keyfile")
	if err != nil {
		return nil, nil, err
	}

	var idht *dht.IpfsDHT
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/udp/4001/quic"),
		libp2p.Identity(priv),
		// enable relay
		libp2p.EnableRelay(),
		// enable node to use relay for wire communication
		libp2p.EnableAutoRelay(autorelay.WithPeerSource(peerChan), autorelay.WithNumRelays(4)),
		// force node belive it is behind a NAT firewall to force using relays
		// libp2p.ForceReachabilityPrivate(),
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
		return nil, nil, errors.WithStack(err)
	}
	return host, idht, nil
}

// run as a goose server in ipfs network
// well.. the server is not actually a ipfs host, instead it talks in goose/0.0.1 protocol
func ServeIPFS(tunnel *tunnel.Tunnel, namespace string) {
	host, err := NewP2PHost()
	if err != nil {
		logger.Fatalf("Error: %+v", err)
	}
	// set server stream hanlder
	host.SetStreamHandler(protocolName, func(s network.Stream) {
		defer s.Close()
		logger.Printf("received new stream %s", s.ID())
	})
	// host.SetAdvertise(true, namespace)
	// do backgroud relay refresh jobs
	if err := host.Background(); err != nil {
		logger.Fatalf("Error: %+v", err)
	}
}

// connect to remote peer once
func connectLoopIPFS(host *P2PHost, p *peer.AddrInfo, localAddr string, tunnel *tunnel.Tunnel) error {

	// connect to the peer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	retries := 1
	for {
		s, err := host.NewStream(ctx, p.ID, protocolName)
		msg := fmt.Sprintf("%+v", err)
		if err != nil && retries > 0 && strings.Contains(msg, transientErrorString) {
			time.Sleep(time.Second * 15)
			logger.Printf("transient connection, try again for %s", p.ID)
			retries -= 1
			continue
		} else if err != nil {
			return errors.WithStack(err)
		}
		defer s.Close()
	}
	return errors.Errorf("failed connect to server, max retries reached")
}

// connect to remote peer by PeerId
func ConnectIPFS(endpoint, localAddr, namespace string, tunnel *tunnel.Tunnel) error {

	host, err := NewP2PHost()
	if err != nil {
		logger.Fatalf("Error: %+v", err)
	}
	// no advertise for client
	// host.SetAdvertise(false, namespace)
	go host.Background()
	for {
		time.Sleep(time.Second * 5)
		// decode peerid
		var p *peer.AddrInfo
		// use certain server
		if endpoint != "" {
			peerID, err := peer.Decode(endpoint)
			if err != nil {
				return errors.WithStack(err)
			}
			p = &peer.AddrInfo{
				ID: peerID,
			}
			logger.Printf("connecting to server %s", p.ID)
			logger.Printf("connection to server %s failed: %+v", p.ID, connectLoopIPFS(host, p, localAddr, tunnel))
			continue
		} else {
			// try find a random server
			ctx, cancel := context.WithTimeout(context.Background(), time.Second * 300)
			search := host.GetAdvertiseName()
			logger.Printf("trying to find a server in namespace: %s\n", search)
			peers, err := host.FindPeers(ctx, search)
			if err != nil {
				cancel()
				return errors.WithStack(err)
			}
			// chose a random peer
			for p := range peers {
				logger.Printf("connecting to server %s", p.ID)
				logger.Printf("connection to server %s failed: %+v", p.ID, connectLoopIPFS(host, &p, localAddr, tunnel))
				time.Sleep(time.Second)
			}
			cancel()
			if p == nil {
				logger.Printf("no servers found\n")
			}
		}
	}
	return nil
}
