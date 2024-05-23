# Decentralized Tunnel Network - Goose

## Features

- **Config-Free Node Discovery**: Eliminates the need for manual configuration by automatically discovering peers in the network. It uses the libp2p network and is bootstrapped via the IPFS network, making the setup hassle-free.

- **Protocol Support**: Offers flexibility by supporting multiple protocols, including QUIC and WireGuard. This allows users to choose the protocol that best suits their needs.

- **Virtual Private Network**: Creates a virtual network interface named `goose`, enabling secure and private communication channels over the internet.

- **Fake-IP**:  Utilizes the `fake-ip` method to selectively route traffic either through the secure tunnel interface or directly to the real network interface. This feature allows for more granular control over traffic routing. Users can write custom scripts to handle the selection of routing, making it highly customizable.

## Usage

Run the following command to see the available options:

```bash
Usage of bin/goose:
  -b string
        bootstraps
  -e string

        comma separated remote endpoints.
        eg. ipfs/QmVCVa7RfutQDjvUYTejMyVLMMF5xYAM1mEddDVwMmdLf4,ipfs/QmYXWTQ1jTZ3ZEXssCyBHMh4H4HqLPez5dhpqkZbSJjh7r

  -f string
        forward networks, comma separated CIDRs
  -g string
        geoip db file
  -l string

        virtual ip address to use in CIDR format.
        local ipv4 address to set on the tunnel interface.
         (default "192.168.89.21/24")
  -n string
        namespace
  -name string
        domain name to use, namespace must be set
  -p string
        fake ip range
  -private
        private network
  -r string
        rule script
  -router
        running in routers
  -wg string
        wireguard config file
```


## Examples

simple explantions:
`-f`: forward. make goose works like a proxy. it can forward traffics to external or internal netowkrs. the target network range is provided through
the -f arguments in comman seperated CIDRs. under the hood. goose create a masquerade rule to the target network. so it wokrs like a proxy to the terget netowrk.
`-router`: if thi sargument is set. goose will not create the masquerade rule in `-f`. this is usefull for running goose in a router which has it's own routing rules.  Lan traffic always gose to the router(gateway). so this masquerade rule is not needed.
`-e`: not really nessassury any more. deprecated
`-l`: set a ipaddress to the `goose` virtual interface. it will set it automatically and randomlly. you may not need to set it manually
`-n`: the namespace, only nodes under the same namespace will discovery each other
`-name`: node name
`-p`: enable fake ip pool, the fake ip method. goose intercept all dns responses if the -p arguments are provied. and it will try to replace the dns answer A record to a fake ip which is alloced from the fake ip pool or range. the ip range is added to your host's routing table (the routing next hot is the `goose` interface) by goose. so all your subsequent request's traffic will go into the goose virtal interface. For example you are behind a restriced network. some Firewall. and you wnat to access google which is block by the firewall. by use goose, you can create a goose interface as a tunnle by pass the firewall, but all you google's traffic need to go throung the goose interface instead of the default interface. that's why goose put all the fake ip range into your host's routing table.
NOTE: fake ip pool works only for 8.8.8.8 as dnsserver. you may need to change your dnsserver to this before running goose

NOTE2: by defaut goose don't change the host's routing table. so any traffic will still goes through the host's original interface. but goose will add the ip range in fake ip pool, and ips learned from other goose node to the host's routing tables. so only ip in the routing table will goes through the tunnel. thus. for access a remote host on local host  through a middle `goose` host. you have to set `-f` on the middle node(contains the remote host ip) and `-p` on local node(if you are to access the remote host by domain, you can ignore the `-p` argument if you are to access the remote host by ip address).

So, as you can see. goose redirects traffice to the `goose` (tunnel interface) by adding routing entries to the host's trouting table. traffics destination ip match the routing rules goes to the tunnel. 

the flowing is what added to the routing table

```
route add [the -p CIDRs] gw [goose interface's gw]
route add [ipranges learnt from other goose node] gw [goose interface's gw]
```
1ipranges learnt from other goose node` includes:  virtual ips assigned on other node's goose interface, ip ranges in other's `-f` arguments. 
each goose node broadcast ip asscigent to it's own goose interface and ip range in the `-f` arguments to other peers. other peers may add this range to thier routing table. that's how this stuff works. the routing broadcast machinism wokrs just like the RIP protocol. implemented in the `pkg/routing/router.go` in function `UpdateRouting`

NOTE3: the above talks about how goose redirect traffic by destination ip. but we always access the websites throuhg domains (DNS). so goose uses the fake-ip method. to intercept all dns response and change the resolved ip to one in the `-p` (fake-ip pool) range. As the fake ip pool is already added to the host's routing table. so the website's traffics goes to the `goose` (tunnel) interface. therefor, to access website thorugh a middle goose node,. you must use the -p argument on your local node. otherwise the dns resolved ip won't match the routing table and it goes to the orignal interface which is not what you want.


`-g` '-r': for handling the fake ip with a java script. to determine whether this dns answer should be replcaced or not. if the rule match it will not be replaced so traffic related to the ip or domain goes though the default interface(not tunneled). otherwise it will be replaced. plus the routing table above `-p`.  thus goes to the `goose` interace(tunneled)

you can always find the details  for fake-ip in the code. under `pkg/routing/fakeip` folder.  repo: https://github.com/nickjfree/goose


before you start working with goose. you need a host with a public ip adrress.
if not. you can use the ipfs network as the bootstrap this will also work.

some public ipfs bootstrap peers. note bootstrap peers shoule be in multiaddress format.
comma seperated
"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
"ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
"/ip4/104.131.131.82/udp/4001/quic-v1/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",


bellow are some setup examples for diffrent use cases.
NOTE: all the examples ignores the  `-b` arguments bootstraps, you must provide the bootstrap peers in realworld use cases. use either your pulblic node or ipfs's node

### Simple Connection

1. On Computer A, run:

```bash
    goose -n my-network -name a
```

2. On Computer B, run:

```bash
    goose -n my-network -name b
```

3. After a few minutes, they will connect. You can ping B from A using:

```bash
ping a.my-network

64 bytes from a.goose.my-network(192.168.0.4): icmp_seq=1 ttl=63 time=188 ms
64 bytes from a.goose.my-network(192.168.0.4): icmp_seq=2 ttl=63 time=206 ms
64 bytes from a.goose.my-network(192.168.0.4): icmp_seq=3 ttl=63 time=748 ms
64 bytes from a.goose.my-network(192.168.0.4): icmp_seq=4 ttl=63 time=562 ms
```

### Network Forwarding

1. Assume Computer A is connected to a private network `10.1.1.0/24`.

2. On Computer A, run:

```bash
    goose -n my-network -name a -f 10.1.1.0/24
```

3. On Computer B, run:

```bash
    goose -n my-network -name b
```

4. Now you can access any host in `10.1.1.0/24` from Computer B using:

```bash
ping 10.1.1.1

64 bytes from 10.1.1.1: icmp_seq=1 ttl=63 time=188 ms
64 bytes from 10.1.1.1: icmp_seq=2 ttl=63 time=206 ms
64 bytes from 10.1.1.1: icmp_seq=3 ttl=63 time=748 ms
64 bytes from 10.1.1.1: icmp_seq=4 ttl=63 time=562 ms
```

### Fake-IP Example

1. On Computer A, run:

```bash
    goose -n my-network -name a -f 0.0.0.0/0
```

2. On Computer B:

####  Custom Script for Routing (Optional)

Use `rule.js` to define custom routing rules.

The custom script must define a `matchDomain(domain)` function. Any traffic that matches the criteria set in this function will bypass the tunnel and be routed directly to the real network interface.

The scripts should be written in ES5

Here's an example:

```javascript
// rule.js
var filters = ['baidu', 'shifen', 'csdn', 'qq', 'libp2p'];  // domain patterns which their dns response's ip result shoule not be replace by fake ip
var filterRegions = ['CN']; // regions which the dns response's ip belogs and this ip shoule not be replace by fake ip if in this region

function isIPv4(str) {
  var ipv4Regex = /^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
  return ipv4Regex.test(str);
}

// Define the main function to match a domain
function matchDomain(domain) {
  if (isIPv4(domain)) {
    var country = getCountry(domain); 
    return filterRegions.indexOf(country) !== -1
  }
  else if (filters.some(function(name) {
    return domain.indexOf(name) !== -1;
  })) {
    return true;
  }
  return false;
}
```
Run the following command to apply the custom rules:

```bash
goose -n my-network -name b -g geoip-country.mmdb -r rule.js -p 11.0.0.0/16
```

Explanation: This command applies the custom routing rules defined in rule.js and sets up a fake-ip range of 11.0.0.0/16.


Testing

```bash
ping www.google.com

PING www.google.com (11.0.0.133) 56(84) bytes of data.
64 bytes from 10.0.0.133 (10.0.0.133): icmp_seq=1 ttl=59 time=188 ms
64 bytes from 10.0.0.133 (10.0.0.133): icmp_seq=2 ttl=59 time=189 ms
64 bytes from 10.0.0.133 (10.0.0.133): icmp_seq=3 ttl=59 time=188 ms
64 bytes from 10.0.0.133 (10.0.0.133): icmp_seq=4 ttl=59 time=188 ms

ping www.baidu.com

PING www.wshifen.com (104.193.88.123) 56(84) bytes of data.
64 bytes from 104.193.88.123 (104.193.88.123): icmp_seq=1 ttl=50 time=150 ms
64 bytes from 104.193.88.123 (104.193.88.123): icmp_seq=2 ttl=50 time=149 ms
64 bytes from 104.193.88.123 (104.193.88.123): icmp_seq=3 ttl=50 time=149 ms
```

### WireGuard Example

WireGuard is a modern, secure, and fast VPN tunnel that aims to be easy to use and lean.

#### Example WireGuard Config File

Below is an example of a WireGuard configuration file that can be used with Goose:

```bash
[Interface]
PrivateKey = mIz7fpuVMc4p1S3e3D4sifkq1fGtgzRJs/kgcuYARWE=
ListenPort = 51820

[Peer]  
PublicKey = CdjruGQqzRC5zUUQEPNjXRPlbmj5t/C0VzF+g93wGkM=
AllowedIPs = 10.0.0.1/32
PersistentKeepalive = 25

PublicKey = x0BPthZpWvmt+KagQgX1zdCQtAHi1Rv6PhcHkOb1cjA=
AllowedIPs = 10.0.0.2/32
PersistentKeepalive = 25

PublicKey = CNx+uklxUet6JQASvh315s1zKqsXh8n1sm3PYUNgeiU=
AllowedIPs = 10.0.0.3/32
PersistentKeepalive = 25
```

#### Running the WireGuard Command

To integrate WireGuard with Goose, run the following command:

```bash
goose -n my-network -name a -wg /etc/wg.conf
```

This command does the following:

- `-n my-network`: Specifies the virtual network name as `my-network`.
- `-name a`: Sets the node name to `a`.
- `-wg /etc/wg.conf`: Points to the WireGuard configuration file located at `/etc/wg.conf`.

#### Connecting to the Virtual Network

After running this command, you can connect to the virtual `my-network` using any WireGuard client implementation.

### Running in router Example

it works the same as other examples. just one thing to note. 

you need to add the `-router` arguments. it means router mode. already explain in the `-route` arguments in this doc.

1. if you are using openwrt for your router firmware. you may need to create a new zone. and put the `goose ` interace into that zone.
and eanble `lan` to `goose` forwardings. `goose` to `lan` forwardings. pictures added

2. you shoule use the `-f` arguments and prived the lan cidr range to it.
this will enalbe bidirectinal comunicating for all your hosts in lan and hosts in the goose virtual network.
