import socket
import threading
import traceback

DATA_SIZE = 1024 * 1024 * 10

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(("127.0.0.1", 10011))
s.listen()

def recvall(conn, n):
    data = b""
    while len(data) < n:
        r = conn.recv(n - len(data))
        if not r:
            return None
        data += r
    return data

def handle(c: socket.socket, addr):
    try:
        r = recvall(c, DATA_SIZE)
        c.sendall(r)
    except Exception as e:
        traceback.print_exc()
    
    c.close()

    print("connection ends", addr)

try:
    while True:
        c, addr = s.accept()

        print("new connection", addr)
        threading.Thread(target=handle, args=(c, addr)).start()
except KeyboardInterrupt as e:
    s.close()
    raise e