# Commonweb2
通过分离上行下行流量来绕过某些防火墙

# 原理
CW2 客户端把每一个连接分离成两个 HTTP 连接，一个上传，一个下载。数据流通过 HTTP chunked encoding (`Transfer-Encoding: chunked`) 来传输。

CW2 服务端通过 HTTP mehtod (GET / POST) 来区分上下行连接。上下行连接根据 `X-Session-ID` 合成一个连接，转发到 `remote` 服务器。

![img](https://github.com/sduoduo233/commonweb2/raw/master/commonweb2.png)

# 编译
1. `git clone https://github.com/sduoduo233/commonweb2.git`
2. `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w"`

# 下载
[Github release](https://github.com/sduoduo233/commonweb2/releases)

# 如何使用
启动服务端:

```
./commonweb2 -mode server -listen 127.0.0.1:56000 -remote 127.0.0.1:56050
```

启动客户端:

```
./commonweb2 -mode client -up http://127.0.0.1:56000/ -down http://127.0.0.1:56000/ -listen 127.0.0.1:56010
```

# 使用 TLS

## 服务端

NGINX 可以用来反向代理 CW2

NGINX 配置示例:

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

## 客户端

把 `-up` 和 `-down` 改成 https 地址

```
./commonweb2 -mode client -up https://example.com/secret_path -down https://example.com/secret_path -listen 127.0.0.1:56010
```

## 使用 UTLS

TLS 指纹可以用来识别使用 Golang 标准库创建的 TLS 连接。某些防火墙可能会使用 TLS 指纹来检测代理服务器，应为 Golang 被代理软件广泛使用。

UTLS 可以用来模仿主流浏览器的 TLS 指纹。

添加 `-utls` 来启用 UTLS

```
./commonweb2 -mode client -up https://example.com/secret_path -down https://example.com/secret_path -listen 127.0.0.1:56010 -utls
```