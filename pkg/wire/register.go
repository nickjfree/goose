package wire

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"github.com/pkg/errors"
	"goose/pkg/tunnel"
)


const (
	// register message types
    TYPE_OK         int8 = iota
    TYPE_FAILED
    TYPE_REGISTER
)


// register ip payload
type RegisterMessagePayload struct {
	// msg type
	MsgType int8
	// client addr
	SrcAddr string
	// server addr
	DstAddr string
	// result
	Result string
}


// do register
func RegisterAddr(w Wire, addr string) (string, error) {

	logger.Printf("start registering addr %s", addr)
	// parse addr
	localIP, _, err := net.ParseCIDR(addr)
	if err != nil {
		return "", errors.Wrap(err, "")
	}		
	req := RegisterMessagePayload{
		MsgType: TYPE_REGISTER,
		SrcAddr: localIP.String(),
		DstAddr: "255.255.255.255",
	}
	payload := bytes.Buffer{}
	enc := gob.NewEncoder(&payload)
	// send register request
	if err := enc.Encode(req); err != nil {
		return "", errors.Wrapf(err, "encode register request error %+v", req)
	}
	if err := w.Write(tunnel.NewTunMessage("", "", payload.Bytes())); err != nil {
		return "", errors.Wrap(err, "send register request error")
	}
	// get response
	msg, err := w.Read()
	if err != nil {
		return "", errors.Wrap(err, "register response error")
	}
	// check response
	buffer, ok := msg.Payload().([]byte)
	if !ok {
		return "", errors.Wrapf(err, "invalid payload type")
	}
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	var resp RegisterMessagePayload
	if err := dec.Decode(&resp); err != nil {
		return "", errors.Wrapf(err, "decode resigter response error %+v", msg.Payload())
	}
	logger.Printf("got server response type(%d) result(%s)", resp.MsgType, resp.Result)
	if resp.MsgType == TYPE_OK {
		logger.Printf("registering addr %s successful: %s, server addr %s", addr, resp.Result, resp.DstAddr)
		return resp.DstAddr, nil
	}
	if resp.MsgType == TYPE_FAILED {
		return msg.GetSrc(), errors.Errorf("register addr faild %s", resp.Result)
	}
	return "", errors.Errorf("unknow response %+v", resp)
}


// handle register
func HandleRegisterAddr(w Wire, t *tunnel.Tunnel) (string, error) {
	
	// read register request
	msg, err := w.Read()
	if err != nil {
		return "", errors.Wrap(err, "read register request error")
	}
	// decode payload
	var req RegisterMessagePayload
	buffer, ok := msg.Payload().([]byte)
	if !ok {
		return "", errors.Wrapf(err, "invalid payload type")
	}
	dec := gob.NewDecoder(bytes.NewReader(buffer))
	if err := dec.Decode(&req); err != nil {
		return "", errors.Wrapf(err, "decode resigter request error %+v", msg.Payload())
	}
	addr := req.SrcAddr
	var resp RegisterMessagePayload
	if req.MsgType == TYPE_REGISTER {
		logger.Printf("got registering request for addr %s", addr)
		if port := t.GetPort(addr); port != nil && !port.Disabled {
			resp.MsgType = TYPE_FAILED
			resp.Result = fmt.Sprintf("address \"%s\" already used by others", addr)
		} else if net.ParseIP(addr) == nil  {
			resp.Result = "invalid address"
			resp.MsgType = TYPE_FAILED
		} else {
			resp.Result = "ok"
			resp.MsgType = TYPE_OK
		}
	} else {
		return "", errors.New("invalid request")
	}
	// return server addr to client
	var serverAddr string
	fallback := t.GetFallbackPort()
	if fallback != nil {
		serverAddr = fallback.GetAddr()
	} else {
		serverAddr = ""
	}
	resp.DstAddr = serverAddr
	// send response
	payload := bytes.Buffer{}
	enc := gob.NewEncoder(&payload)
	// send register request
	if err := enc.Encode(resp); err != nil {
		return "", errors.Wrapf(err, "encode register response error %+v", resp)
	}
	if err := w.Write(tunnel.NewTunMessage("", "", payload.Bytes())); err != nil {
		return "", errors.Wrap(err, "send register request error")
	}
	logger.Printf("registering response for addr %s is (%s)", addr, resp.Result)
	if resp.MsgType != TYPE_OK {
		return "", errors.New(resp.Result)
	}
	return addr, nil
}
