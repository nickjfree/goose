<!-- ABOUT THE PROJECT -->


## About The Project
[![Makefile CI](https://github.com/fengjian/goose/actions/workflows/makefile.yml/badge.svg)](https://github.com/fengjian/goose/actions/workflows/makefile.yml)

Wow!  
Such proxy    
Much QUIC

An ipv4 tunnel proxy using QUIC as tranport protocol.

## How it works

- Details:  
![How it works][howitworks]

<!-- GETTING STARTED -->
## Getting Started


```
$ goose -h
Usage of bin/goose:
  -c    run as client
  -e string
        remote http endpoint (default "https://us.nick12.com")
  -local string
        local ipv4 address to set on the tunnel interface (default "192.168.100.1/24")
  -p string
         protocol (default "http3")
```

### server

```sh
$ goose -local 192.168.100.1/24
```
setup server ip address and nat
```sh
$ ip link set goose up
$ ip addr add 192.168.100.1/24 dev goose
$ iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
```
### client

```sh
$ goose -c -e https://realserverip:55556 -local 192.168.100.2/24
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


<!-- ROADMAP -->
## Roadmap

* Automatically config ip address and routing table
* Supporting other protocols

[howitworks]: images/howitworks.jpg
[logically]: images/virtual.jpg
