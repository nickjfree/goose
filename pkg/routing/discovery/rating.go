package discovery

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/ipfs/go-ipns"
	"github.com/pkg/errors"

	"goose/pkg/wire/ipfs"
)

// monitoring the goose network and get current infomation
// then calculate ratings based on that information
type RatingSystem struct {
	// p2p host
	*ipfs.P2PHost
	// suggested address for this node
	suggestedAddress net.IP
	// local address pool to use
	localNet net.IPNet
	// current like and dislikes
	ratings map[string]Rating
	// lock
	lock sync.Mutex
}

type Rating struct {
	// peerID
	PeerID string `json:"peerID,omitempty"`
	// addresses seen by me
	Networks []string `json:"networks,omitempty"`
	// score
	Score int `json:"score,omitempty"`
}

func NewRatingSystem(localNet net.IPNet, address net.IP) *RatingSystem {
	m := &RatingSystem{
		P2PHost:          ipfs.GetP2PHost(),
		suggestedAddress: address,
		localNet:         localNet,
		ratings:          make(map[string]Rating),
	}
	go m.run()
	return m
}

func (m *RatingSystem) Rate(peerID string, network net.IPNet, metric int) {
	m.lock.Lock()
	defer m.lock.Unlock()
	r, ok := m.ratings[peerID]
	if ok {
		for _, n := range r.Networks {
			if n == network.String() {
				return
			}
		}
		r.Networks = append(r.Networks, network.String())
		r.Score = r.Score + 8/metric
	} else {
		r = Rating{
			PeerID:   peerID,
			Networks: []string{network.String()},
			Score:    8 / metric,
		}
	}
	m.ratings[peerID] = r
}

func (m *RatingSystem) refresh() error {

	myID := m.ID()
	// get keys
	privateKey := m.Peerstore().PrivKey(myID)
	publicKey := m.Peerstore().PubKey(myID)

	// rate for myself
	me := net.IPNet{
		IP:   m.suggestedAddress,
		Mask: net.CIDRMask(32, 32),
	}
	selfRating := Rating{
		PeerID:   myID.String(),
		Networks: []string{me.String()},
	}

	// create ratings for self and connected peers
	m.lock.Lock()
	m.ratings[myID.String()] = selfRating
	data, err := json.Marshal(m.ratings)
	if err != nil {
		return errors.WithStack(err)
	}
	ratingPretty, err := json.MarshalIndent(m.ratings, "", "  ")
	if err != nil {
		return errors.WithStack(err)
	}

	// clear ratings for next roud
	m.ratings = make(map[string]Rating)
	m.lock.Unlock()

	// create an IPNS record that expires in 300s about the ratings
	ipnsRecord, err := ipns.Create(privateKey, data, 0, time.Now().Add(300*time.Second), 300*time.Second)
	if err != nil {
		return errors.WithStack(err)
	}
	if err := ipns.EmbedPublicKey(publicKey, ipnsRecord); err != nil {
		return errors.WithStack(err)
	}
	data, err = ipnsRecord.Marshal()
	if err != nil {
		return errors.WithStack(err)
	}

	// put value to libp2p network
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()
	if err := m.PutValue(ctx, ipns.RecordKey(myID), data); err != nil {
		return errors.WithStack(err)
	}
	logger.Printf("set ipns recored %s\n%s", myID.String(), string(ratingPretty))
	return nil
}

func (m *RatingSystem) run() {

	// refresh the network's repuation status every 120s
	ticker := time.NewTicker(time.Second * 120)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.refresh(); err != nil {
				logger.Println("failed refresh reputaions", err)
			}
		}
	}
}
