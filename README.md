# comm
Go package for scalable and high-performance communications

## Overview

Go is a powerful programming language with a lot of potential. Unfortunately,
the Go ecosystem is not as mature as the C ecosystem (nothing wrong with it,
it is just a fact) and lack basic building blocks for system software
development.

I personally am a big fan on projects such as CCI (https://github.com/CCI/cci)
and OpenUCX (https://github.com/openucx/ucx), which I use all the time when
working on a C project. Unfortunately, there are no similar packages for Go.
And yes, it is possible to interface Go and C to rely on these projects, but
it is also true that having similar software in Go is a huge advantage for
consistency.

This is why I decided to work on this project: a communication package in Go
that gives me most of the features I need when working on my pet projects
(and hopefully more than that in the future).

The key capabilities I need are (and not all are yet implemented):
- A scalable solution: be able to support hundreds of endpoints on any
given node and make sure the connections between these endpoints are
performed in a scalable manner (e.g., not necessarily one connection
for any pair of endpoints, a.k.a., support multiplexing).
- Support both explicit configurations (e.g., I *want* to use TCP) and
implicit configurations (e.g., when coding, I *do not want* to assume
where the endpoints will be, they can be on the same node, find the
way to connect them).
- Do not assume any underlying protocol but instead find the best one
to connect a pair of endpoint. If an Infiniband network is available,
the appropriate protocol should be used; if the two endpoints are on
the same node, a shared memory transport should be used. In theory,
we can support any protocol, including TCP, UDP, IB, shared memory,
http, IP, Ethernet.

## Core concepts

Core concepts in this communication package.

### Communication engine

A communication engine is what gathers and *orchestrates* the infrastrcture.
As such, it is for instance discovering the local resource in 'auto' mode
and ensure that the communication layer is configured appropriately to
connect a pair of endpoints with minimum assumption about the configuration.

In the MPI universe, this would be similar to COMM_WORLD but simpler and
more flexible (the functional model is simply different).

An application using this package can have more than one engine. No
communication is possible without a communication engine. Communications
cannot cross the communication engine boundary, i.e., communications
between engines is *not* supported.

### Endpoints

An endpoint is an object that can send and receive messages using a
communication engine. Endpoint are created before any connection is
created. Any endpoint can rely on one or several transports to reach
another endpoint.

### Transports & Concrete transports

The concept of transport hides the specificities of the underlying
networking protocols used for communication (e.g., TCP, share memory).
The engine provides an abstract concept of transports, which provides
a set of semantics and interfaces for connecting endpoints and perform
communications. All transports are, upon connection to a remote endpoint,
associated with a unique *concrete transport*. A *concrete transport* is
a transport that implements the transport interface and semantics for 
a given networking protocol. For exmaple, *TCPTransport* is the concrete
transport for TCP.

