import socket
import threading
import os
import hashlib
import traceback

DATA_SIZE = 1024 * 1024 * 10
data = os.urandom(DATA_SIZE) # test data

print(len(data))

def sha256sum(b):
    s = hashlib.sha256()
    s.update(b)
    return s.hexdigest()


# sha256 of test data
sha256test = sha256sum(data)


# read at most n bytes from conn
def recvall(conn, n):
    data = b""
    while len(data) < n:
        r = conn.recv(n - len(data))
        if not r: return data
        data += r
    return data

def worker():
    print("worker started")
    while True:
        try:
            
            s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s.connect(("127.0.0.1", 10066))

            s.sendall(data) # send test data to server

            r = recvall(s, DATA_SIZE) # server should echo back all of out data
            sha256recv = sha256sum(r)

            localAddr = s.getsockname()

            s.close()

            if len(r) != DATA_SIZE:
                # missing data
                if sha256recv == sha256sum(data[:len(r)]):
                    print("missing data, sha256 match", DATA_SIZE - len(r), localAddr)
                else:
                    print("missing data, sha256 mismatch", DATA_SIZE - len(r), localAddr)
            else:
                if sha256recv != sha256test:
                    print("sha256 mismatch")
            
        except Exception as e:
            traceback.print_exc()

for _ in range(10):
    threading.Thread(target=worker).start()