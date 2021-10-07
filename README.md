# gocdn

`gocdn` consists of a root http server and a matching cdn server binary. The root server serves a directory over http 
like a normal http webserver. When starting the cdn server, it registers itself at the root server. Now when the root 
server gets a request, it randomly redirects the client to one of the registered cdn servers. The cdn server might 
not have the requested file in cache, so it requests the root server with the header `X-Cdn-Request` set, which 
causes the root server to always serve the file itself and does not redirect the request to another cdn server.
On the next request, this cdn server can serve the file from its local cache, thus taking over work and bandwidth from 
the root server.

## Installing

### Go binary

```
go get gocdn
```

### Docker Container

TODO:

```
docker pull ...
```

## Running

### Go binaries

First start the root server:

```
go run root_server -serve-dir . -self-serve .html -addr :8192
```

Then start the cdn server(s):

```
go run cdn_server --listen-addr :8193 -remote-addr http://localhost:8193 -root-addr http://localhost:8192
```

### Docker container

First start the root server:

```
docker run -p 8192:8192 root_server -serve-dir . -self-serve .html -addr :8192
```

Then start the cdn server(s):

```
docker run cdn_server -p 8192:8192 -listen-addr :8193 -remote-addr http://localhost:8193 -root-addr http://localhost:8192
```

> Note that the first argument of `-p` is the host port, which must match with `-remote-addr` and the second is the 
> container port, which must match with `--listen-addr`.