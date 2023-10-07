# Commonweb2
通过上下行分流来绕过某些防火墙

# 原理
客户端把每一个连接拆分成两个 HTTP 连接，一个上传，一个下载。HTTP 连接使用 `Transfer-Encoding: chunked` 来传输未知长度的 body。

服务端使用 HTTP Method (GET / POST) 来区分上下行，使用 `X-Session-Id` Header 区分 session。

![img](https://github.com/sduoduo233/commonweb2/raw/master/commonweb2.png)

# 如何使用
服务端启动参数:

```
./commonweb2 -mode server -listen 127.0.0.1:56000 -remote 127.0.0.1:56050
```

客户端启动参数:

```
./commonweb2 -mode client -up http://127.0.0.1:56000/ -down http://127.0.0.1:56000/ -listen 127.0.0.1:56010
```