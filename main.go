package main

import (
	"commonweb2/client"
	"commonweb2/server"
	"flag"
	"log/slog"
	"os"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	mode := flag.String("mode", "server", "server / client")
	up := flag.String("up", "http://127.0.0.1:56000/", "[client only] upload url")
	down := flag.String("down", "http://127.0.0.1:56000/", "[client only] download url")
	remote := flag.String("remote", "127.0.0.1:56200", "[server only]ex remote address")
	listen := flag.String("listen", "127.0.0.1:56100", "listen addresss")
	flag.Parse()

	if *mode != "server" && *mode != "client" {
		panic("invalid mode")
	}

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))

	slog.Info("commonweb2", "mode", *mode)

	if *mode == "server" {

		s := server.NewServer(*listen, *remote)
		err := s.Start()
		if err != nil {
			slog.Error("start server", "error", err)
		}

	} else {

		c := client.NewClient(*up, *down, *listen)
		err := c.Start()
		if err != nil {
			slog.Error("start client", "error", err)
		}

	}

}
