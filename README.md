
<!-- ABOUT THE PROJECT -->
<h2 align="center">A Decentralized  Tunnel Network</h2>

[![Build](https://github.com/nickjfree/goose/actions/workflows/build.yml/badge.svg)](https://github.com/nickjfree/goose/actions/workflows/build.yml/badge.svg)

Very tunnel

An ipv4 tunnel proxy using  many transport protocols

## Architecture
![How it works][arch]

<!-- GETTING STARTED -->
## Getting Started


```
$ goose -h
Usage of bin/goose:
  -e string

        comma separated remote endpoints.

        for http/http3 protocols. this should be a http url of the goose server.
        for ipfs protocols, this should be a libp2p PeerID. If empty, the client will try to find a random goose server in the network

  -f string
        forward networks, comma separated CIDRs
  -l string

        virtual ip address to use in CIDR format.
        local ipv4 address to set on the tunnel interface.
         (default "192.168.100.2/24")
  -n string
        namespace
```
[arch]: images/arch.png
