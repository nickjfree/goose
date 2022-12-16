# go-libp2p roadmap Q4’22/Q1’23 <!-- omit in toc -->

```
Date: 2022-10-20
Status: Accepted
Notes: Internal go-libp2p stakeholders have aligned on this roadmap. Please add any feedback or questions in:
https://github.com/libp2p/go-libp2p/issues/1806
```

## Table of Contents <!-- omit in toc -->

- [About the Roadmap](#about-the-roadmap)
  - [Vision](#vision)
  - [Sections](#sections)
  - [Done criteria](#done-criteria)
- [Benchmarking and Testing](#benchmarking-and-testing)
- [🛣️ Milestones](#️-milestones)
  - [2022](#2022)
    - [Early Q4 (October)](#early-q4-october)
    - [Mid Q4 (November)](#mid-q4-november)
    - [End of Q4 (December)](#end-of-q4-december)
  - [2023](#2023)
    - [Early Q1 (January)](#early-q1-january)
    - [Mid Q1 (February)](#mid-q1-february)
    - [End of Q1 (March)](#end-of-q1-march)
  - [Up Next](#up-next)
- [📖 Appendix](#-appendix)
  - [A. 📺 Universal Browser Connectivity](#a--universal-browser-connectivity)
    - [1. WebRTC: Browser to Server](#1-webrtc-browser-to-server)
    - [2. WebRTC: Browser to Browser](#2-webrtc-browser-to-browser)
    - [3. WebTransport: Update to new draft versions](#3-webtransport-update-to-new-draft-versions)
  - [B. ⚡️ Handshakes at the Speed of Light](#b-️-handshakes-at-the-speed-of-light)
    - [1. Early Muxer Negotiation](#1-early-muxer-negotiation)
    - [2. Adding security protocol](#2-adding-security-protocol)
    - [3. 0.5 RTT data optimization](#3-05-rtt-data-optimization)
  - [C. 🧠 Smart Dialing](#c--smart-dialing)
    - [1. Happy Eyeballs](#1-happy-eyeballs)
    - [2. QUIC Blackhole detector](#2-quic-blackhole-detector)
    - [3. RTT estimation](#3-rtt-estimation)
  - [D. 📊 Comprehensive Metrics](#d--comprehensive-metrics)
  - [E. 📢 Judicious Address Advertisements](#e--judicious-address-advertisements)

## About the Roadmap

### Vision
We, the maintainers, are committed to upholding libp2p's shared core tenets and ensuring go-libp2p is: [**Secure, Stable, Specified, and Performant.**](https://github.com/libp2p/specs/blob/master/ROADMAP.md#core-tenets)

Roadmap items in this document were sourced in part from the [overarching libp2p project roadmap.](https://github.com/libp2p/specs/blob/master/ROADMAP.md)

### Sections
This document consists of two sections: [Milestones](#️-milestones) and the [Appendix](#-appendix)

[Milestones](#️-milestones) is our best educated guess (not a hard commitment) around when we plan to ship the key features.
Where possible projects are broken down into discrete sub-projects e.g. project "A" may contain two sub-projects: A.1 and A.2

A project is signified as "complete" once all of it's sub-projects are shipped.

The [Appendix](#-appendix) section describes a project's high-level motivation, goals, and lists sub-projects.

Each Appendix header is linked to a GitHub Epic. Latest information on progress can be found in the Epics and child issues.

### Done criteria
The "Definition of Done" for projects/sub-projects that involve writing new protocols/ modify existing ones usually consist of the following:
- If a specification change is required:
    - [ ] Spec is merged and classified as "Candidate Recommendation"
    - [ ] (by virtue of the above) At least one major reference implementation exists
- [ ] A well established testing criteria is met (defined at the outset of the project including but not limited to testing via Testground, compatibility tests with other implementations in the Release process, etc.)
- [ ] Public documentation (on docs.libp2p.io) exists

Supporting projects (such as testing or benchmarking) may have different criteria.

## Benchmarking and Testing
As mentioned in our [vision](#vision), performance and stability are core libp2p tenets. Rigorous benchmarking and testing help us uphold them. Related projects are listed in the [libp2p/test-plans roadmap](https://github.com/libp2p/test-plans/blob/master/ROADMAP.md) and the [testground/testground roadmap](https://github.com/testground/testground/blob/master/ROADMAP.md).
Our major priorities in Q4’22 and Q1’23 are:
- [interoperability testing](https://github.com/libp2p/test-plans/issues/53) (across implementations & versions and between transports, muxers, & security protocols)
- performance [benchmark go-libp2p using Testground](https://github.com/testground/testground/pull/1425) (create a benchmark suite to run in CI, create a public performance dashboard, [demonstrate libp2p is able to achieve performance on par with HTTP](https://github.com/libp2p/test-plans/issues/27))

These projects are parallel workstreams, weighed equally with roadmap items in this document. Some efforts like interoperability testing have a higher priority than implementation projects. The go-libp2p maintainers co-own these efforts with the js-libp2p, rust-libp2p, and Testground maintainers.

[**Click here to see the shared Q4’22/Q1’23 testing and benchmarking priorities.**](https://github.com/libp2p/test-plans/blob/master/ROADMAP.md)

## 🛣️ Milestones
### 2022

#### Early Q4 (October)
- [B.1 ⚡ Early Muxer Negotiation](#1-early-muxer-negotiation)

#### Mid Q4 (November)
- [***➡️ test-plans/Interop tests for all existing/developing libp2p transports***](https://github.com/libp2p/test-plans/blob/master/ROADMAP.md#2-interop-test-plans-for-all-existingdeveloping-libp2p-transports)
- [***➡️ test-plans/Benchmarking using nix-builders***](https://github.com/libp2p/test-plans/blob/master/ROADMAP.md#1-benchmarking-using-nix-builders)

#### End of Q4 (December)
- [A.1 📺 WebRTC Browser -> Server](#1-webrtc-for-browser-to-server)
- [C.1 🧠 Happy Eyeballs](#1-happy-eyeballs)
- [D 📊 Swarm Metrics](#e--comprehensive-metrics)

### 2023

#### Early Q1 (January)
- [B.2 ⚡ Adding security protocol](#2-adding-security-protocol)
- [C.2 🧠 QUIC Blackhole detector](#2-quic-blackhole-detector)

#### Mid Q1 (February)
- [C.3 🧠 RTT estimation](#3-rtt-estimation)
  - 🎉 Estimated Project Completion

#### End of Q1 (March)
- [B.3 ⚡ 0.5 RTT data optimization (for QUIC)](#3-05-rtt-data-optimization)
  -  🎉 Estimated Project Completion
- [***➡️ test-plans/Benchmarking using remote runners***](https://github.com/libp2p/test-plans/blob/master/ROADMAP.md#2-benchmarking-using-remote-runners)

### Up Next
- [A.2 📺 WebRTC: Browser to Browser](#2-webrtc-browser-to-browser)
- [A.3 📺 WebTransport: Update to new draft versions](#3-webtransport-update-to-new-draft-versions)
- [***➡️ test-plans/Expansive protocol test coverage***](https://github.com/libp2p/test-plans/blob/master/ROADMAP.md#d-expansive-protocol-test-coverage)
- [E 📢 Judicious Address Advertisements](#f--judicious-address-advertisements)

## 📖 Appendix

**Projects are listed in descending priority.**

### [A. 📺 Universal Browser Connectivity](https://github.com/libp2p/go-libp2p/issues/1811)

**Why**: A huge part of “the Web” is happening inside the browser. As a universal p2p networking stack, libp2p needs to be able to offer solutions for browser users.

**Goal**: go-libp2p ships with up-to-date WebTransport and (libp2p-) WebRTC implementations, enabled by default. This allows connections between browsers and public nodes, browsers and non-public nodes, as well as two browsers.

#### 1. [WebRTC: Browser to Server](https://github.com/libp2p/go-libp2p/pull/1655)
Add support for WebRTC transport in go-libp2p, enabling browser connectivity with servers. This will cover the browsers that don't support WebTransport (most notable is iOS Safari). This is getting close to finalized.
#### 2. WebRTC: Browser to Browser
A follow up to A.1 where we will begin the work to specify the semantics of browser to browser connectivity and then implement it in go-libp2p.
#### 3. [WebTransport: Update to new draft versions](https://github.com/libp2p/go-libp2p/issues/1717)
As the protocol is still under development by IETF and W3C, the go-libp2p implementation needs to follow. We have a dependency on Chrome to support the new draft version of WebTransport protocol. To stay up to date, we will have to move as soon as Chrome ships supports the new draft version.

### [B. ⚡️ Handshakes at the Speed of Light](https://github.com/libp2p/go-libp2p/issues/1807)

**Why**: Historically, libp2p has been very wasteful when it comes to round trips spent during connection establishment. This is slowing down our users, especially their TTFB (time to first byte) metrics.

**Goal**: go-libp2p optimizes its handshake latency up to the point where only increasing the speed of light would lead to further speedups. In particular, this means:

#### 1. [Early Muxer Negotiation](https://github.com/libp2p/specs/issues/426)
Cutting off the 1 RTT wasted on muxer negotiation
#### 2. [Adding security protocol](https://github.com/libp2p/specs/pull/353)
Cutting off the 1 RTT wasted on security protocol negotiation by including the security protocol in the multiaddr
#### 3. 0.5 RTT data optimization
Using 0.5-RTT data (for TLS) / a Noise Extension to ship the list of Identify protocols, cutting of 1 RTT that many protocols spend waiting on `IdentifyWait`

### [C. 🧠 Smart Dialing](https://github.com/libp2p/go-libp2p/issues/1808)

**Why**: Having a large list of transports to pick from is great. Having an advanced stack that can dial all of them is even greater. But dialing all of them at the same time wastes our, the network’s and the peer’s resources. 

**Goal**: When given a list of multiaddrs of a peer, go-libp2p is smart enough to pick the address that results in the most performant connection (for example, preferring QUIC over TCP), while also picking the address such that maximizes the likelihood of a successful handshake.

#### 1. [Happy Eyeballs](https://github.com/libp2p/go-libp2p/issues/1785)
Implement some kind of “Happy-Eyeballs” style prioritization among all supported transports
#### 2. QUIC Blackhole detector
Detection of blackholes, especially relevant to detect UDP (QUIC) blackholing
#### 3. RTT estimation
Estimation of the expected RTT of a connection based on two nodes’ IP addresses, so that Happy Eyeballs Timeouts can be set dynamically

### [D. 📊 Comprehensive Metrics](https://github.com/libp2p/go-libp2p/issues/1356)

**Why**: For far too long, go-libp2p has been a black box. This has hurt us many times, by allowing trivial bugs to go undetected for a long time ([example](https://github.com/ipfs/kubo/pull/8750)). Having metrics will allow us to track the impact of performance improvements we make over time.

**Goal**: Export a wider set of metrics across go-libp2p components and enable node operators to monitor their nodes in production. Optionally provide a sample Grafana dashboard similar to the resource manager dashboard.

**How**: This will look similar to how we already expose resource manager metrics. Metrics can be added incrementally for libp2p’s components. First milestone is having metrics for the swarm.


### [E. 📢 Judicious Address Advertisements](https://github.com/libp2p/go-libp2p/issues/1812)

**Why**: A node that advertises lots of addresses hurts itself. Other nodes will have to try dialing a lot of addresses before they find one that actually works, dramatically increasing handshake latencies.

**Goal**: Nodes only advertise addresses that they are actually reachable at.

**How**: Unfortunately, the AutoNAT protocol can’t be used to probe the reachability of any particular address (especially due to a bug in the go-libp2p implementation deployed years ago). Most likely, we need a second version of the AutoNAT protocol.

Related discussion: [https://github.com/libp2p/go-libp2p/issues/1480](https://github.com/libp2p/go-libp2p/issues/1480)
