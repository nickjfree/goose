package ipfs


// import (
// 	"testing"
// 	"context"
// 	"sync"
// 	"time"
// 	"goose/pkg/wire"
// )

// // test dial ipfs wire
// func TestIPFSWireConnect(t *testing.T) {

// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	// inbount channel reader
// 	go func () {
// 		defer wg.Done()
// 		ctx, cancel := context.WithTimeout(context.Background(), time.Second * 120)
// 		defer cancel()
// 		select {
// 		case w, _ := <- wire.Out():
// 			t.Logf("outbound wire %+v", w)
// 			defer w.Close()
// 		case <- ctx.Done():
// 			t.Log("wait for wire timedout")
// 			t.Fail()
// 		}
// 	} ()
// 	if err := wire.Dial("ipfs", "QmVQMTSS8xLigftqzvSQenzfRQtiT7UTjJiKnqWotod5iP"); err != nil {
// 		t.Logf("%+v", err)
// 		t.Fail()
// 	}
// 	wg.Wait()
// }


// // test inbound ipfs wire
// func TestIPFSWireInbount(t *testing.T) {

// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	// inbount channel reader
// 	go func () {
// 		defer wg.Done()
// 		ctx, cancel := context.WithTimeout(context.Background(), time.Second * 120)
// 		defer cancel()
// 		select {
// 		case w, _ := <- wire.In():
// 			t.Logf("outbound wire %+v", w)
// 			defer w.Close()
// 		case <- ctx.Done():
// 			t.Log("wait for wire timed out")
// 			t.Fail()
// 		}
// 	} ()
// 	wg.Wait()
// }