

<!-- ABOUT THE PROJECT -->
<h2 align="center"># Decentralized Tunnel Network - Goose</h2>

[![Build](https://github.com/nickjfree/goose/actions/workflows/build.yml/badge.svg)](https://github.com/nickjfree/goose/actions/workflows/build.yml/badge.svg)

Welcome to Goose, a decentralized tunnel network built on top of libp2p. With Goose, every node is both a client and server, meaning there's no need for a specific server. This makes it easier to establish connections between nodes.
  
    


## Architecture
Here's how Goose works:
![How it works][arch]

<!-- GETTING STARTED -->
## Getting Started

### Connecting to each other

Nodes connect to each other automatically if they are started with the same namespace. Assuming you have two hosts, A and B, that are located in different LANs, you can connect them using Goose. First, start `node A` with the virtual address `192.168.0.3` on host A:

```
goose -n name1 -l 192.168.0.3/24
```
Then, start `node B` with the virtual address `192.168.0.4` on host B:
```
goose -n name1 -l 192.168.0.4/24
```

Now, both nodes are connected to each other within the same virtual network `192.168.0.0/24`.

To ping `node B` from host A:
```
ping 192.168.0.4
PING 192.168.0.4 (192.168.0.4) 56(84) bytes of data.
64 bytes from 192.168.0.4: icmp_seq=1 ttl=63 time=188 ms
64 bytes from 192.168.0.4: icmp_seq=2 ttl=63 time=206 ms
64 bytes from 192.168.0.4: icmp_seq=3 ttl=63 time=748 ms
64 bytes from 192.168.0.42: icmp_seq=4 ttl=63 time=562 ms
```

### Forwarding to Real Networks

If nodes only communicate under their virtual networks, it's not very useful. To enable communication with real networks, you can run `node B` with the `-f` argument:

```
goose -n test -l 192.168.0.4/24 -f 10.1.1.0/24
```
After this, processes on host A can communicate with any host under `10.1.1.0/24`.

To ping some host  in `10.1.1.0/24`:
```
ping 10.1.1.3
PING 10.1.1.3 (10.1.1.3) 56(84) bytes of data.
64 bytes from 10.1.1.3: icmp_seq=1 ttl=63 time=188 ms
64 bytes from 10.1.1.3: icmp_seq=2 ttl=63 time=206 ms
64 bytes from 10.1.1.3: icmp_seq=3 ttl=63 time=748 ms
64 bytes from 10.1.1.3: icmp_seq=4 ttl=63 time=562 ms
```
[arch]: images/arch.png

## What You Can Do With It

Goose can be used for various purposes such as:

-   Creating a private network for IoT devices
-   Setting up a virtual private network (VPN)
-   Establishing a secure, private communication channel between remote teams
-   Creating a private gaming network
-   Enabling secure communication between multiple devices within a smart home ecosystem


The possibilities are endless, and Goose provides a flexible and powerful platform for building decentralized, secure networks for a wide range of use cases.