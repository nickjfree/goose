package discovery

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/pkg/errors"

	"github.com/nickjfree/goose/pkg/wire/ipfs"
)

const (
	// advertise node prefix
	prefixGooseNode = "/goose/0.2.0/node"
	// advertise network prefix
	prefixGooseNetwork = "/goose/0.2.0/network"
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
	ns []string
	// peer channel
	peers chan string
}

func nodeKey(ns string) string {
	return fmt.Sprintf("%s/%s", prefixGooseNode, ns)
}

func NewPeerFinder(namesapce string) PeerFinder {

	namespaces := strings.Split(namesapce, ",")
	ns := []string{}
	for i := range namespaces {
		ns = append(ns, nodeKey(namespaces[i]))
	}
	pf := PeerFinder{
		P2PHost: ipfs.GetP2PHost(),
		ns:      ns,
		peers:   make(chan string),
	}
	go pf.start()
	return pf
}

func (pf *PeerFinder) Peers() <-chan string {
	return pf.peers
}

func (pf *PeerFinder) findPeers() error {

	count := 0
	for _, ns := range pf.ns {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
		defer cancel()

		logger.Printf("searching peers in %s", ns)
		peers, err := pf.FindPeers(ctx, ns)
		if err != nil {
			return errors.WithStack(err)
		}
		for p := range peers {
			// remove self
			if p.ID == pf.ID() {
				continue
			}
			pf.AllowPeer(p.ID.String())
			pf.peers <- fmt.Sprintf("ipfs/%s", p.ID)
			logger.Printf("found peer %s in %s", p.ID, ns)
			count += 1
		}
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

	for _, ns := range pf.ns {
		if _, err := pf.Advertise(ctx, ns, discovery.TTL(advertiseInterval)); err != nil {
			logger.Println("failed to advertise", err)
		}
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
			for _, ns := range pf.ns {
				if _, err := pf.Advertise(ctx, ns, discovery.TTL(advertiseInterval)); err != nil {
					logger.Println("failed to advertise", err)
				}
			}
		}
	}
}
