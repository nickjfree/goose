package connector


import (
	"log"
	"net"
	"os"

	"goose/pkg/wire"
)


var (
	logger = log.New(os.Stdout, "ipfswire: ", log.LstdFlags | log.Lshortfile)
)


func RunTestLoop() {
	for {
		select {
		case w := <- wire.In():
			logger.Printf("new in bound wire %+v", w)
			msg := wire.Message{}
			if err := w.Decode(&msg); err != nil {
				logger.Printf("closed wire %+v %+v", w, err)
				w.Close()
			}
			logger.Printf("got wire message %+v", msg)
			// response
			msg = wire.Message{
				Type: wire.MessageTypeRouting,
				Payload: wire.Routing{
					Type: wire.RoutingRegisterOK,
					Routings: []net.IPNet{},
				},
			}
			if err := w.Encode(&msg); err != nil {
				logger.Printf("closed wire %+v %+v", w, err)
				w.Close()
			}
		case w := <- wire.Out():
			logger.Printf("new out bound wire %+v", w)
			// send routing message test
			msg := wire.Message{
				Type: wire.MessageTypeRouting,
				Payload: wire.Routing{
					Type: wire.RoutingRegister,
					Routings: []net.IPNet{},
				},
			}
			if err := w.Encode(&msg); err != nil {
				logger.Printf("closed wire %+v %+v", w, err)
				w.Close()
			}
			if err := w.Decode(&msg); err != nil {
				logger.Printf("closed wire %+v %+v", w, err)
				w.Close()
			}
			logger.Printf("got wire message %+v", msg)
		}
	}
}

