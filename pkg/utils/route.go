package utils

import (
	"sync"
	"time"
)

var RouteTable *HostRoute

func init() {
	RouteTable = &HostRoute{
		rules:   make(map[string]route),
		actions: make(chan route),
	}
	// handle route actions
	go RouteTable.Start()
}

// route entry
type route struct {
	target  string
	gateway string
	ref     int
}

// host route tables
type HostRoute struct {
	// lock
	mu sync.Mutex
	// route tables
	rules map[string]route
	// route action
	actions chan route
}

// set route
func (h *HostRoute) SetRoute(target, gateway string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if gateway == "" {
		gateway = defaultGateway
	}
	r, ok := h.rules[target]
	if ok {
		r.ref += 1
		r.gateway = gateway
	} else {
		r = route{
			target:  target,
			gateway: gateway,
			ref:     1,
		}
	}
	h.rules[target] = r
	// notify route change
	h.actions <- r
	return nil
}

// delete route
func (h *HostRoute) RemoveRoute(target string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rules[target]
	if ok {
		r.ref -= 1
		if r.ref <= 0 {
			delete(h.rules, target)
			// notify route change
			h.actions <- r
		} else {
			h.rules[target] = r
		}
	}
	return nil
}

// refresh route
func (h *HostRoute) Start() error {

	// refresh route every 2 min
	ticker := time.NewTicker(time.Second * 120)
	defer ticker.Stop()

	for {

		select {
		// handle route update and delete
		case r := <-h.actions:
			if r.ref <= 0 {
				// delete route
				logger.Printf("delete host route %s -> %s", r.target, r.gateway)
				if err := RemoveRoute(r.target, r.gateway); err != nil {
					logger.Printf("error set route %s", err)
				}
			} else {
				// update route
				logger.Printf("update host route %s -> %s", r.target, r.gateway)
				if err := SetRoute(r.target, r.gateway); err != nil {
					logger.Printf("error set route %s", err)
				}
			}
		// refresh
		case <-ticker.C:
			h.mu.Lock()
			for _, r := range h.rules {
				logger.Printf("update host route %s -> %s", r.target, r.gateway)
				if err := SetRoute(r.target, r.gateway); err != nil {
					logger.Printf("error set route %s", err)
				}
			}
			h.mu.Unlock()
		}
	}
}
