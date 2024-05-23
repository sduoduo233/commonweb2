# Stress test 2

## How to run this test

0. `go build`

1. Start commonweb2 client

```
./commonweb2 -mode client -up http://127.0.0.1:20000 -down ht
tp://127.0.0.1:20000 -listen 127.0.0.1:10066
```

2. Start commonweb2 server

```
./commonweb2 -mode server -listen 127.0.0.1:20000 -remote 127.0.0.1:10011
```

3. Start test server

```
python3 server.py
```

4. Start test client

```
python3 client.py
```