# Twitch notes (ported from Xtra, checked on 2026-11-14)

Reference: Xtra (AGPL-3.0), `repository/GraphQLRepository.kt`, `repository/PlayerRepository.kt`,
`ui/login/LoginActivity.kt`, `util/chat/*`, `src/main/graphql/*.graphql`.

## GraphQL

- Endpoint `POST https://gql.twitch.tv/gql`, JSON body, header `Client-Id`.
- Anonymous client id used by Xtra by default: `kd1unb4b3q4t58fwlpcbzcbnm76a8fp`. Raw queries work with it without an
  integrity token (checked: a `streams` query answered).
- Authenticated calls add `Authorization: OAuth <token>` and must use the client id the token was issued to.
- Device code login refuses the anonymous id (`invalid client`, checked 2026-11-15). Xtra logs in with the Twitch
  Android app id `ue6666qo983tsx6so1t0vnawi233wa` and keeps it for every call made with that token: same here.
- Playback tokens add a random `X-Device-Id` (16 random bytes, hex).
- Raw queries (Xtra `.graphql` files): `streams`, `games`, `user(login)`, `users(logins)`, `user.videos`,
  `user.clips`, `video(id)`, `clip(slug)` (with `assets.videoQualities` and `playbackAccessToken`), `searchUsers`,
  `searchStreams`, `searchCategories`, `searchFor(index: VOD)`, `game(slug).streams|videos|clips`,
  `user.followedLiveUsers` (needs the user token).

## Persisted queries (sha256 hashes)

- `PlaybackAccessToken`: `ed230aa1e33e07eebb8928504583da78a5173989fadfb1ac94be06a04f3cdbe9`
  variables `isLive`, `login`, `isVod`, `vodID`, `playerType` (`site`), `platform` (`web`).
  Answer: `data.streamPlaybackAccessToken|videoPlaybackAccessToken.{value,signature}`. `value` is JSON with
  `expires` (unix seconds, about 20 min), `server_ads`, `hide_ads`, `subscriber`, `turbo`, `geoblock_reason`, ...
- `sendChatMessage`: `0435464292cf380ed4b3d905e4edcb73078362e82c06367a5b2181c76c822fa2`.

## Usher

- Live: `https://usher.ttvnw.net/api/v2/channel/hls/<login>.m3u8` with `allow_source=true`, `allow_audio_only=true`,
  `fast_bread=true`, `include_unavailable=true`, `p=<random>`, `platform=web`, `sig`, `token`, optional
  `supported_codecs`. Offline channel: 404.
- VOD: `https://usher.ttvnw.net/vod/v2/<id>.m3u8` with the same params, without `fast_bread`.
- Master playlist: `#EXT-X-SESSION-DATA` lines leak `USER-IP`, `USER-COUNTRY` and node names: drop them.
  Variants carry `STABLE-VARIANT-ID` / `IVS-NAME` (`1080p60`, `720p60`, ..., `audio_only`).
- Hosts seen: playlists `*.playlist.ttvnw.net`, segments `*.hls.ttvnw.net` (cloudfront) and `*.hls.live-video.net`.
- Media playlist: 2 s segments, `#EXTINF:2.002,live` for live content, `#EXT-X-PROGRAM-DATE-TIME`,
  `#EXT-X-TWITCH-PREFETCH:<url>` (low latency), `#EXT-X-DATERANGE` lines. The `twitch-trigger` daterange carries a
  beacon URL (`X-TV-TWITCH-TRIGGER-URL`): drop it.

## Ads (SSAI)

- Ad daterange: `ID="stitched-ad-..."`, or `CLASS="twitch-stitched-ad"`, or any `X-TV-TWITCH-AD-*` attribute.
- Ad segments have an `#EXTINF` title other than `live`.
- Xtra hides them by switching to a second stream (other token / proxy) until the ad ends ("video swap").

## Login (device code)

- `POST https://id.twitch.tv/oauth2/device`, form `client_id=<gql id>&scopes=channel_read chat:read user_blocks_edit
  user_blocks_read user_follows_edit user_read` -> `device_code`, `user_code`, `verification_uri`, `interval`,
  `expires_in`.
- User types the code on `https://www.twitch.tv/activate`.
- Poll `POST https://id.twitch.tv/oauth2/token` form `client_id`, `device_code`,
  `grant_type=urn:ietf:params:oauth:grant-type:device_code` -> `access_token`, `refresh_token`.
- `GET https://id.twitch.tv/oauth2/validate` with `Authorization: OAuth <token>`; revoke on
  `POST https://id.twitch.tv/oauth2/revoke`.

## Chat

- `wss://irc-ws.chat.twitch.tv`, send `CAP REQ :twitch.tv/tags twitch.tv/commands`, then anonymous
  `NICK justinfan<1000-9999>` (or `PASS oauth:<token>` + `NICK <login>`), `JOIN #<login>`, answer `PING` with `PONG`.
