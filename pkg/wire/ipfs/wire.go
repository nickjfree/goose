// connect to server through ipfs network
package ipfs

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/libp2p/go-libp2p"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"
	dis_routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/quic-go/quic-go"

	"github.com/nickjfree/goose/pkg/message"
	"github.com/nickjfree/goose/pkg/options"
	"github.com/nickjfree/goose/pkg/wire"
	"github.com/pkg/errors"
)

// ipfs bootstrap node
var (
	bootstraps = []string{
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		// "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		// "/ip4/104.131.131.82/udp/4001/quic-v1/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}
)

const (
	// connection protection tag
	connectionTag = "goose"
	// protocol name
	protocolName = "/goose/0.2.0"
	// transiend error string
	transientErrorString = "transient connection"
	// key size
	keyBits = 2048
)

var (
	logger = log.New(os.Stdout, "ipfswire: ", log.LstdFlags|log.Lshortfile)

	// manager
	ipfsWireManager *IPFSWireManager
)

// register ipfs wire manager
func init() {

	if len(options.Bootstraps) > 0 {
		bootstraps = strings.Split(options.Bootstraps, ",")
	}

	ipfsWireManager = newIPFSWireManager()
	wire.RegisterWireManager(ipfsWireManager)
}

func isP2PCircuitAddress(addr ma.Multiaddr) bool {
	for _, p := range addr.Protocols() {
		if p.Name == "p2p-circuit" {
			return true
		}
	}
	return false
}

// hack. get quic.Connection to send datagram
func getQuicConn(c network.Conn) quic.Connection {

	// wrappedCapableConn := reflect.ValueOf(c).Elem().FieldByName("conn").Elem()
	// capableConn := wrappedCapableConn.FieldByName("CapableConn").Elem()

	// capableConn = capableConn.Elem()

	// quicConn := capableConn.FieldByName("quicConn")
	// logger.Printf("type is %s value is %+v", reflect.TypeOf(c), c)

	quicConn := reflect.ValueOf(c).Elem().FieldByName("conn").Elem().Elem().FieldByName("quicConn")
	v := reflect.NewAt(quicConn.Type(), unsafe.Pointer(quicConn.UnsafeAddr())).Elem()
	return v.Interface().(quic.Connection)
}

// ipfs wire
type IPFSWire struct {
	// base
	wire.BaseWire
	// stream
	s network.Stream
	// quic connection
	conn quic.Connection
	// close func
	closeFunc func() error
}

func (w *IPFSWire) Endpoint() string {
	return fmt.Sprintf("ipfs/%s", w.s.Conn().RemotePeer())
}

func (w *IPFSWire) Address() net.IP {
	peerAddr := w.s.Conn().RemoteMultiaddr()
	ip, _ := peerAddr.ValueForProtocol(ma.P_IP4)
	return net.ParseIP(ip)
}

// Encode
func (w *IPFSWire) Encode(msg *message.Message) error {
	var err error
	var msgs []message.Message
	// routings message may exceed MTU, split it
	if msg.Type == message.MessageTypeRouting {
		msgs, err = msg.Split()
		if err != nil {
			return err
		}
	} else {
		// traffic message, risk of exceeding MTU
		// TODO: fix this, can we lower the MTU of the tunnel interface?
		msgs = []message.Message{*msg}
	}
	for _, msg := range msgs {
		buf, err := msg.Encode()
		if err != nil {
			return err
		}
		if err := w.conn.SendDatagram(buf); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

// Decode
func (w *IPFSWire) Decode(msg *message.Message) error {
	buf, err := w.conn.ReceiveDatagram(context.Background())
	if err != nil {
		return errors.WithStack(err)
	}
	if err := msg.Decode(buf); err != nil {
		return err
	}
	return nil
}

// send message to ipfs wire
func (w *IPFSWire) Close() error {
	w.closeFunc()
	return nil
}

// IPFS wire manager
type IPFSWireManager struct {
	wire.BaseWireManager
	*P2PHost
}

func GetP2PHost() *P2PHost {
	return ipfsWireManager.P2PHost
}

func newIPFSWireManager() *IPFSWireManager {
	host, err := NewP2PHost()
	if err != nil {
		logger.Fatalf("Error: %s", err)
	}
	// do background relay refresh jobs
	go host.Background()
	m := &IPFSWireManager{
		P2PHost:         host,
		BaseWireManager: wire.NewBaseWireManager(),
	}
	// set server stream handler
	m.SetStreamHandler(protocolName, func(s network.Stream) {
		host.ConnManager().Protect(s.Conn().RemotePeer(), connectionTag)
		// close func
		close := func() error {
			// unprotect connecttion
			host.ConnManager().Unprotect(s.Conn().RemotePeer(), connectionTag)
			// close stream
			s.Close()
			// we use quic datagram, also close the connection
			s.Conn().Close()
			return nil
		}
		// read the hello
		buf := make([]byte, 32)
		if _, err := s.Read(buf); err != nil {
			close()
			logger.Printf("error reading client hello %s", err)
			return
		}
		// ignore unlimit relay connections
		if isP2PCircuitAddress(s.Conn().RemoteMultiaddr()) {
			close()
			logger.Printf("ignore unlimited relay %+v", s.Conn())
			return
		}
		logger.Printf("received new stream(%s) peerId (%s) data %s", s.ID(), s.Conn().RemotePeer(), string(buf))
		// got an inbound wire
		m.In <- &IPFSWire{
			s:         s,
			conn:      getQuicConn(s.Conn()),
			closeFunc: close,
		}
	})
	return m
}

func (m *IPFSWireManager) Dial(endpoint string) error {

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
		msg := fmt.Sprintf("%s", err)
		if err != nil && retries > 0 && strings.Contains(msg, transientErrorString) {
			logger.Printf("transient connection, try again for %s", p.ID)
			time.Sleep(time.Second * 15)
			retries -= 1
			continue
		} else if err != nil {
			cancel()
			return errors.WithStack(err)
		}
		m.ConnManager().Protect(s.Conn().RemotePeer(), connectionTag)
		// close func
		close := func() error {
			// unprotect connecttion
			m.ConnManager().Unprotect(s.Conn().RemotePeer(), connectionTag)
			// close stream
			s.Close()
			// we use quic datagram, also close the connection
			s.Conn().Close()
			// cancel stream context
			cancel()
			return nil
		}
		// send hello to make sure there is only one stream bettwen 2 peers
		if _, err := s.Write([]byte("hello")); err != nil {
			close()
			return errors.WithStack(err)
		}
		// ignore unlimit relay connections
		if isP2PCircuitAddress(s.Conn().RemoteMultiaddr()) {
			close()
			return errors.Errorf("ignore unlimited relay %+v", s.Conn())
		}
		// got an outbound wire
		m.Out <- &IPFSWire{
			s:         s,
			conn:      getQuicConn(s.Conn()),
			closeFunc: close,
		}
		return nil
	}
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
	// dht
	dht *dht.IpfsDHT
	// peer info channel for auto relay
	peerChan chan peer.AddrInfo
	// host context
	ctx context.Context
	// cancel
	cancel context.CancelFunc
	// advertise namespace
	namespace string
	// allowedlist of peers
	allowedPeers map[string]ma.Multiaddr
}

func NewP2PHost() (*P2PHost, error) {
	// create peer chan
	peerChan := make(chan peer.AddrInfo, 100)

	// this is the callback for AutoRelay
	peerSource := func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
		c := make(chan peer.AddrInfo, 100)
		go func() {
			for {
				select {
				case <-ctx.Done():
					// AutoRelay is satisfied, close the channel
					close(c)
					return
				case peer := <-peerChan:
					c <- peer
				}
			}
		}()
		return c
	}
	// create p2p host
	host, dht, err := createHost(peerSource)
	if err != nil {
		return nil, err
	}
	routingDiscovery := dis_routing.NewRoutingDiscovery(dht)
	ctx, cancel := context.WithCancel(context.Background())
	h := &P2PHost{
		Host:             host,
		RoutingDiscovery: routingDiscovery,
		dht:              dht,
		ctx:              ctx,
		peerChan:         peerChan,
		cancel:           cancel,
		allowedPeers:     make(map[string]ma.Multiaddr),
	}
	if err := h.Bootstrap(bootstraps); err != nil {
		return nil, err
	}
	return h, nil
}

// bootstrap with public peers
func (h *P2PHost) Bootstrap(peers []string) error {
	// bootstrap timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
	defer cancel()

	if len(peers) < 1 {
		logger.Printf("not enough bootstrap peers")
		return nil
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
				logger.Printf("failed to bootstrap with %s: %s", p.ID, err)
				errs <- err
				return
			}
			logger.Printf("bootstrapped with %s", p.ID)
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

	// state ticker
	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	// bootstrap ticker
	bootstrapTicker := time.NewTicker(time.Second * 900)
	defer bootstrapTicker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return nil
		case <-ticker.C:
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
				case <-h.ctx.Done():
					return nil
				default:
					continue
				}
			}
			logger.Printf("%d peers(ipfs)", len(peerList))
			addrText, err := json.MarshalIndent(h.Addrs(), "", "  ")
			if err != nil {
				logger.Printf("error %s", errors.WithStack(err))
			}
			logger.Printf("peerid: %s\naddrs: %s\n", h.ID(), addrText)
		case <-bootstrapTicker.C:
			// bootstrap refesh
			if err := h.Bootstrap(bootstraps); err != nil {
				logger.Printf("bootstrap error %s", err)
			}
		}
	}
}

// add peer to allowlist of resource manager
func (h *P2PHost) AllowPeer(peer string) error {
	addr := fmt.Sprintf("/ip4/0.0.0.0/ipcidr/0/p2p/%s", peer)
	if _, ok := h.allowedPeers[addr]; ok {
		return nil
	} else {
		al := rcmgr.GetAllowlist(h.Host.Network().ResourceManager())
		maAddr := ma.StringCast(addr)
		if err := al.Add(maAddr); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (h *P2PHost) PutValue(ctx context.Context, key string, value []byte, opts ...routing.Option) (err error) {
	return h.dht.PutValue(ctx, key, value, opts...)
}

func (h *P2PHost) GetValue(ctx context.Context, key string, opts ...routing.Option) ([]byte, error) {
	return h.dht.GetValue(ctx, key, opts...)
}

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
func createHost(peerSource func(ctx context.Context, numPeers int) <-chan peer.AddrInfo) (host.Host, *dht.IpfsDHT, error) {

	folder := fmt.Sprintf("data/%s", strings.ReplaceAll(options.Namespace, "-", "_"))
	if err := os.MkdirAll(folder, 0644); err != nil {
		return nil, nil, errors.WithStack(err)
	}
	keyPath := fmt.Sprintf("%s/keyfile", folder)
	priv, err := getPrivKey(keyPath)
	if err != nil {
		return nil, nil, err
	}

	// resource manager
	limits := getResourceLimits()
	mgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits))
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	var idht *dht.IpfsDHT
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/udp/4001/quic-v1"),
		libp2p.Identity(priv),
		// enable relay
		libp2p.EnableRelay(),
		// enable node to use relay for wire communication
		libp2p.EnableAutoRelay(autorelay.WithPeerSource(peerSource), autorelay.WithNumRelays(4), autorelay.WithMinCandidates(1)),

		libp2p.EnableRelayService(),
		// force node believe it is behind a NAT firewall to force using relays
		// libp2p.ForceReachabilityPrivate(),
		// hole punching
		libp2p.EnableHolePunching(),
		libp2p.ResourceManager(mgr),
		// must disable metrics, because metrics is buggy
		libp2p.DisableMetrics(),

		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		// enable routing
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			ctx := context.Background()
			idht, err = dht.New(ctx, h, dht.Mode(dht.ModeServer))
			if err = idht.Bootstrap(ctx); err != nil {
				logger.Fatal(err)
			}
			return idht, err
		}),
	}
	host, err := libp2p.New(opts...)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	return host, idht, nil
}
