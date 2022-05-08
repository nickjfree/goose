
<!-- ABOUT THE PROJECT -->
## About The Project

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
  -c    run as client
  -e string
        remote endpoint, http url or peerid
  -local string
        local ipv4 address to set on the tunnel interface (default "192.168.100.1/24")
  -p string
        protocol: http/http3/ipfs (default "ipfs")
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

enable up forwarding
```sh
sysctl -w net.ipv4.ip_forward=1
sysctl -p
```

- Result:  
![logically][logically]

## Decentralized

The newly added p2p protocls. two peers can connect to each other though libp2p's holepuching service.


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
 			
	

## Server Dicovery

No need to setup servers anymore. 
Client will try to search for a server 
```sh
	goose -c -local 192.168.0.2/24
```

[howitworks]: images/howitworks.jpg
[logically]: images/virtual.jpg
