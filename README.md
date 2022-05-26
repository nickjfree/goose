

<!-- ABOUT THE PROJECT -->

## About The Project
[![Build](https://github.com/nickjfree/goose/actions/workflows/build.yml/badge.svg)](https://github.com/nickjfree/goose/actions/workflows/build.yml/badge.svg)

Very tunnel

An ipv4 tunnel proxy using  many transport protocols

## How it works

- Details:  
![How it works][howitworks]

<!-- GETTING STARTED -->
## Getting Started


```
$ goose -h
Usage of goose:
  -c    flag. run as client. if not set, it will run as a server
  -e string

        remote server endpoint.

        for http/http3 protocols. this should be a http url of the goose server.
        for ipfs protocols, this should be a libp2p PeerID. If empty, the client will try to find a random goose server in the network

  -local string

        virtual ip address to use in CIDR format.

        local ipv4 address to set on the tunnel interface.
        if the error message shows someone else is using the same ip address, please change it to another one
         (default "192.168.100.1/24")
  -n string
        namespace
  -p string

        transport protocol.

        options: http/http3/ipfs

        http:
                Client and server communicate through an upgraded http1.1 protocol. (HTTP 101 Switching Protocol)
                Can be used with Cloudflare
        http3:
                Client and server communicate through HTTP3 stream
                faster then http1.1 but doesn't support Cloudflare for now
        ipfs:
                Client and server communicate through a libp2p stream.
                With some cool features:
                Server discovery, client can search for random servers to connect through the public IPFS network
                Hole puching service, client and server can both run behind their NAT firewalls. NO PUBLIC IP NEEDED
         (default "ipfs")
```

### server

```sh
$ goose -local 192.168.100.1/24 -p http3
```
setup server ip address and nat
```sh
$ ip link set goose up
$ ip addr add 192.168.100.1/24 dev goose
$ iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
```

enable ip forwarding
```sh
sysctl -w net.ipv4.ip_forward=1
sysctl -p
```

### client

```sh
$ goose -c -e https://realserverip:55556 -local 192.168.100.2/24 -p http3
```
setup ip address and routing
```sh
$ ip link set goose up
$ ip addr add 192.168.100.2/24 dev goose
$ route add -host <realserverip> gw <oldgateway>
$ route add -net 0.0.0.0/0 gw 192.168.100.1
```


- Result:  
![logically][logically]

## Decentralized

With the newly added p2p protocls. two peers can connect to each other though libp2p's holepuching service.


Peer A
```sh
  goose -local 192.168.0.1/24
```
After obtain peerA 's  id in the  console ouput
PeerB can connect to  peerA. using peer A's id
```sh
  goose -c -local 192.168.0.2/24 -e QmUiFDEvY49nbU86VVDV7q8UUFFRrbAZhrCEGDh32Vb5A1
```

 **No public servers needed**. 
      
  

## Server Discovery

You can setup server within a namespace

server1
```sh
  goose -n us
```
server2
```sh
  goose -n hk
```

Client will try to search for a server  in that namespace

client
```sh
  goose -c -local 192.168.0.2/24 -n us
```

[howitworks]: images/howitworks.jpg
[logically]: images/virtual.jpg
