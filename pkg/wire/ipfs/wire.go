// connect to server through ipfs network
package ipfs

import (
	// "bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net"
	// "io"
	"io/ioutil"
	"log"
	"encoding/json"
	"strings"
	"time"
	"crypto/rand"
	// "path/filepath"
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

	"github.com/pkg/errors"
	"goose/pkg/wire"
	"goose/pkg/message"
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
	logger = log.New(os.Stdout, "ipfswire: ", log.LstdFlags | log.Lshortfile)

	// manager
	ipfsWireManager *IPFSWireManager
)


// register ipfs wire manager
func init() {
	ipfsWireManager = newIPFSWireManager()
	wire.RegisterWireManager(ipfsWireManager)
}

// ipfs wire
type IPFSWire struct {
	// base
	wire.BaseWire
	// stream
	s network.Stream
	// address
	address net.IP
	// encoder and decoer
	encoder *gob.Encoder 
	decoder *gob.Decoder

	// close func
	closeFunc func () error
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
	if err := w.encoder.Encode(msg); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// Decode
func (w *IPFSWire) Decode(msg *message.Message) error {
	if err := w.decoder.Decode(msg); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// send message to ipfs wire
func (w *IPFSWire) Close() (error) {
	return w.closeFunc()
}


// IPFS wire manager
type IPFSWireManager struct {
	wire.BaseWireManager
	*P2PHost
}

func newIPFSWireManager() *IPFSWireManager {
	host, err := NewP2PHost()
	if err != nil {
		logger.Fatalf("Error: %+v", err)
	}
	// do backgroud relay refresh jobs
	go host.Background()
	m := &IPFSWireManager{
		P2PHost: host,
		BaseWireManager: wire.NewBaseWireManager(),
	}
	// set server stream hanlder
	m.SetStreamHandler(protocolName, func(s network.Stream) {
		host.ConnManager().Protect(s.Conn().RemotePeer(), connectionTag)
		logger.Printf("received new stream(%s) peerId (%s)", s.ID(), s.Conn().RemotePeer())
		// close func
		close := func () error {
			// unprotect connecttion
			host.ConnManager().Unprotect(s.Conn().RemotePeer(), connectionTag)
			// close stream
			s.Close()
			return nil
		}
		// got an inbound wire
		m.In <- &IPFSWire{
			s: s,
			closeFunc: close,
			encoder: gob.NewEncoder(s),
			decoder: gob.NewDecoder(s),
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
		m.Out <- &IPFSWire{
			s: s,
			closeFunc: close,
			encoder: gob.NewEncoder(s),
			decoder: gob.NewDecoder(s),
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
		dht: dht,
		ctx: ctx,
		peerChan: peerChan,
		cancel : cancel,
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

	// state ticker
	ticker := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	// advertise ticker
	advertiseTicker := time.NewTicker(time.Hour * 6)
	defer advertiseTicker.Stop()
	// bootstrap ticker
	bootstrapTicker := time.NewTicker(time.Second * 900)
	defer bootstrapTicker.Stop()

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
		case <- bootstrapTicker.C:
			// bootstrap refesh
			if err := h.Bootstrap(bootstraps); err != nil {
				logger.Printf("bootstrap error %+v", err)
			}
		}
	}
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
