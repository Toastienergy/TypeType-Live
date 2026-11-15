# Adding Twitch to a TypeType instance

TypeType-Live is optional. Without it nothing changes: the server answers `404 twitch_disabled` on `/twitch/*` and
the web hides the Twitch entry.

## 1. Run the service

```yaml
  typetype-live:
    image: typetype-live:latest
    environment:
      LIVE_SECRET: ${LIVE_SECRET}            # shared with the server, 32+ characters
      LIVE_SIGNING_KEY: ${LIVE_SIGNING_KEY}  # keeps signed HLS links valid across restarts, 32+ characters
      LIVE_PUBLIC_BASE: /live
      LIVE_SEGMENT_CACHE: 48MiB              # about channels watched x segment size x 6
      GOMEMLIMIT: 100MiB
    read_only: true
    cap_drop: [ALL]
    security_opt: [no-new-privileges:true]
    mem_limit: 128m
    pids_limit: 128
```

Do not publish its port: only the TypeType server (API) and the web container (`/live/hls/`) talk to it.

## 2. Point the server at it

```
TWITCH_SERVICE_URL=http://typetype-live:8080
TWITCH_SERVICE_SECRET=<same value as LIVE_SECRET>
TWITCH_TOKEN_ENCRYPTION_KEY=<32+ random characters>
```

`TWITCH_SERVICE_SECRET` and `TWITCH_TOKEN_ENCRYPTION_KEY` also accept `_FILE` variants. The linked Twitch tokens are
stored encrypted with that key in the `twitch_accounts` table.

## 3. Route the HLS links in the web container

The web image already contains `location ^~ /live/hls/`. Set `TYPETYPE_LIVE_HOST` and `TYPETYPE_LIVE_PORT` if the
service is not reachable as `typetype-live:8080`. Only the signed HLS endpoints are exposed this way; the `/v1` API
stays private behind the shared secret.

## What viewers get

- Lives, VODs and clips played through the instance: Twitch never sees the viewer IP, the browser never sees a
  Twitch URL.
- Stitched ads replaced by a clean backup stream when Twitch offers one, otherwise cut out.
- Chat through one shared IRC connection per channel.
- Linking a Twitch account with a code on twitch.tv/activate: followed lives, subscriber lives, ad free playback for
  Turbo and subscribers, and sending chat messages.
