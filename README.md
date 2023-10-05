# Commonweb2
通过上下行分流来绕过某些防火墙

# 如何使用
服务端启动参数:

```
./commonweb2 -mode server -listen 127.0.0.1:56000 -remote 127.0.0.1:56050
```

客户端启动参数:

```
./commonweb2 -mode client -up http://127.0.0.1:56000/ -down http://127.0.0.1:56000/ -listen 127.0.0.1:56010
```