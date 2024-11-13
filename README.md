# ATProto Logger

This program will connect to the a Jetstream websocket and log out all of the events it sees. It essentially just logs out every event on the ATProtocol network.

This program has no actual usage, it's just cool to see all the events.

## Usage

By default, the program connects to a public Bluesky hosted Jetstream instance (`jetstream1.us-west.bsky.network`), but you can change this by modifying the `wsURL` variable on line `16` in `main.go`. If you want to run your own Jetstream instance for whatever reason, you can do so by following the [Jetstream installation instructions](https://github.com/bluesky-social/jetstream).

Before you start, make sure you have Go (1.23+) installed, (it may work for older versions, idk).

To start the program, run:

```bash
go get
go run main.go
```

Then just enjoy the logs!

## License

Licensed under the MIT License. See [LICENSE](LICENSE) for details.
