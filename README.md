# Commonweb2
Separating uploading and downloading traffic to bypass some firewalls

[简体中文](https://github.com/sduoduo233/commonweb2/blob/master/README_zh_cn.md)

# How does it work
CW2 client separates every connection into two HTTP connections, one for uploading and one for downloading. Data streams are transmitted using HTTP chunked encoding (`Transfer-Encoding: chunked`)

The CW2 server differentiates between uploading and downloading connections using the HTTP method (GET or POST). Connections are reassembled according to the `X-Session-Id` header. Reassembled connections are then forwarded to the `remote` server (e.g. your shadowsocks/vmess server).

![img](https://github.com/sduoduo233/commonweb2/raw/master/commonweb2.png)

# Compile
1. `git clone https://github.com/sduoduo233/commonweb2.git`
2. `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w"`

# Download
[Github release](https://github.com/sduoduo233/commonweb2/releases)

# How to use
Start CW2 server:

```
./commonweb2 -mode server -listen 127.0.0.1:56000 -remote 127.0.0.1:56050
```

Start CW2 client:

```
./commonweb2 -mode client -up http://127.0.0.1:56000/ -down http://127.0.0.1:56000/ -listen 127.0.0.1:56010
```

# Using with TLS

## CW2 server

NGINX can be used to reverse proxy CW2

Example NGINX configuration:

```
server {
  listen 443 ssl;

  ...

  location /secret_path {
    proxy_http_version 1.1;
    
    # add these two lines to enable streaming
    proxy_buffering off;
    proxy_request_buffering off;

    # replace with your CW2 server address
    proxy_pass http://127.0.0.1:8080/;
  }
}
```

## CW2 client

Change `-up` and `-down` parameters to https addresses

```
./commonweb2 -mode client -up https://example.com/secret_path -down https://example.com/secret_path -listen 127.0.0.1:56010
```