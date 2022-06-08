package tun


import (
	"testing"
	"context"
	"sync"
	"time"
	"goose/pkg/wire"
)

// test dial ipfs wire
func TestTunWireConnect(t *testing.T) {

	wires := []wire.Wire{}
	var wg sync.WaitGroup
	wg.Add(1)
	// inbount channel reader
	go func () {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second * 5)
		defer cancel()
		for {
			select {
			case w, _ := <- wire.Out():
				t.Logf("outbound wire %+v", w)
				wires = append(wires, w)
			case <- ctx.Done():
				t.Log("wait for wire timed out")
				return
			}
		}	
	} ()
	if err := wire.Dial("tun", "goose1/192.168.100.2/24"); err != nil {
		t.Logf("%+v", err)
		t.Fail()
	}
	if err := wire.Dial("tun", "goose2/192.168.100.3/24"); err != nil {
		t.Logf("%+v", err)
		t.Fail()
	}
	wg.Wait()
	if len(wires) != 2 {
		t.Logf("wire count not matched %+v", wires)
		t.Fail()
	}
}
