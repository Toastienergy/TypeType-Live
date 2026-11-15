<div align="center">
  <img src="https://raw.githubusercontent.com/TypeType-Video/TypeType/main/assets/banner.svg" alt="TypeType" width="100%">
  <h1>TypeType Live</h1>
  <p>Twitch for TypeType: lives, VODs, clips and chat through the instance.</p>
</div>

You want to know the current position of TypeType about AI ? Go check [this](https://github.com/TypeType-Video/TypeType/blob/dev/AI_TRANSPARENCY.md).

Twitch for TypeType. A Go service that TypeType-Server calls to browse Twitch, play lives, VODs and clips, and use
the chat. It is optional: an instance without it works as before.

## What it does

- Talks to Twitch the way Xtra does (GQL and Usher).
- Rewrites every HLS playlist so segments go through signed links on the instance. The viewer's browser never
  contacts Twitch.
- Ads: switches to an ad-free backup stream when Twitch offers one, otherwise cuts the ad segments.
- Chat: one anonymous IRC connection per channel, shared with every viewer over SSE. Sending needs a linked account.
- Account linking with a code on twitch.tv/activate. The server keeps the token, encrypted.

## Run

```sh
docker build -t typetype-live .
```

| Variable | Default | |
| --- | --- | --- |
| `LIVE_SECRET` | required | Shared with the server, 32+ characters |
| `LIVE_SIGNING_KEY` | random | Keeps signed links valid across restarts |
| `LIVE_PUBLIC_BASE` | | Public prefix of the HLS links, e.g. `/live` |
| `LIVE_SEGMENT_CACHE` | `32MiB` | About channels watched x segment size x 6 |
| `LIVE_ADDR` | `:8080` | |
| `LIVE_LOG_LEVEL` | `info` | |

Do not publish the port. Only the server and the web container talk to it.

## Develop

```sh
./check.sh
```

Needs Go 1.27, gofumpt, golangci-lint, govulncheck and deadcode.

## Thanks

[Xtra](https://github.com/crackededed/Xtra): the Twitch logic here is ported from it.

## License

See [LICENSE](LICENSE).
