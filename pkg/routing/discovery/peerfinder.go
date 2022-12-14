package discovery

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/pkg/errors"

	"goose/pkg/wire/ipfs"
)

const (
	// advertise node [refix]
	prefixGooseNode = "/goose/0.1.0/node"
	// advertise network prefix
	prefixGooseNetwork = "/goose/0.1.0/network"
	// intervals
	searchInterval = time.Second * 3600

	advertiseInterval = time.Second * 900
)

var (
	logger = log.New(os.Stdout, "discovery: ", log.LstdFlags|log.Lshortfile)
)

type PeerFinder struct {
	// p2p host
	*ipfs.P2PHost
	// namespace
	ns string
	// peer channel
	peers chan string
}

func nodeKey(ns string) string {
	return fmt.Sprintf("%s/%s", prefixGooseNode, ns)
}

func NewPeerFinder(ns string) PeerFinder {
	pf := PeerFinder{
		P2PHost: ipfs.GetP2PHost(),
		ns:      nodeKey(ns),
		peers:   make(chan string),
	}
	go pf.start()
	return pf
}

func (pf *PeerFinder) Peers() <-chan string {
	return pf.peers
}

func (pf *PeerFinder) findPeers() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
	defer cancel()
	logger.Printf("searching peers in %s", pf.ns)
	peers, err := pf.FindPeers(ctx, pf.ns)
	if err != nil {
		return errors.WithStack(err)
	}
	count := 0
	for p := range peers {
		// remove self
		if p.ID == pf.ID() {
			continue
		}
		pf.AllowPeer(p.ID.String())
		pf.peers <- fmt.Sprintf("ipfs/%s", p.ID)
		logger.Printf("found peer %s in %s", p.ID, pf.ns)
		count += 1
	}
	logger.Printf("found %d peers(goose)", count)
	return nil
}

func (pf *PeerFinder) start() error {
	// search ticker
	searchTicker := time.NewTicker(searchInterval)
	defer searchTicker.Stop()
	// advertise ticker
	advertiseTicker := time.NewTicker(advertiseInterval)
	defer advertiseTicker.Stop()

	ctx := context.Background()

	if _, err := pf.Advertise(ctx, pf.ns, discovery.TTL(advertiseInterval)); err != nil {
		logger.Println("failed to advertise", err)
	}
	if err := pf.findPeers(); err != nil {
		logger.Println("failed find peers", err)
	}

	// loop
	for {
		select {
		case <-searchTicker.C:
			if err := pf.findPeers(); err != nil {
				logger.Println("failed find peers", err)
			}
		case <-advertiseTicker.C:
			if _, err := pf.Advertise(ctx, pf.ns, discovery.TTL(advertiseInterval)); err != nil {
				logger.Println("failed to advertise", err)
			}
		}
	}
}
