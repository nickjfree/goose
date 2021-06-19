<!-- ABOUT THE PROJECT -->
## About The Project

Wow!  
Such QUIC  
Much proxy  

<!-- GETTING STARTED -->
## Getting Started

### server
```sh
$ goose --local 192.168.100.1/24

```
setup server as nat gateway
```sh
$ ip link set tun1 up
$ ip addr add 192.168.100.1/24 dev goose
$ iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
```

### client:
```sh
$ goose -c -http3 https://serverip -local 192.168.100.2/24
```
set ip address
```sh
$ ip link set goose up
$ ip addr add 192.168.100.2/24 dev goose
```

