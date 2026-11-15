# Local performance runs (2026-11-14, dev machine, fake Twitch, 512 KiB segments)

Fake Twitch (`tools/faketwitch`) on loopback, service binary on loopback, `tools/loadgen` as viewers (each viewer:
API call, master, then the variant every 2 s and every new segment). Peak RSS is `VmHWM` of the service process.

| viewers / channels | cache | GOMEMLIMIT | requests | failures | p99 playlist | p99 segment | served | upstream segment fetches | peak RSS |
|---|---|---|---|---|---|---|---|---|---|
| 100 / 10 | 32 MiB | 96 MiB | 7 200 | 0 | 5.26 ms | 8.56 ms | 157 MB/s | (loadgen dedupe bug) | 95 MB |
| 500 / 10 | 16 MiB | 56 MiB | 53 500 | 0 | 1.58 ms | 4.80 ms | 786 MB/s | (loadgen dedupe bug) | 58 MB |
| 500 / 10 | 24 MiB | 56 MiB | 20 975 | 0 | 1.42 ms | 4.30 ms | 218 MB/s | 4 732 for 12 475 served | 60 MB |
| 500 / 10 | 64 MiB, 30 s TTL | 96 MiB | 15 000 | 0 | - | 3.95 ms | 197 MB/s | 150 for 7 500 served | 100 MB |
| 500 / 10 | 40 MiB, 12 s TTL | 72 MiB | 18 485 | 0 | 1.70 ms | 3.89 ms | 175 MB/s | 204 for 9 985 served | 76 MB |

Notes:
- The first two rows refetched every segment on each refresh (sealed links were not stable yet), so they measure raw
  relay throughput rather than a realistic player.
- With a cache large enough for the live window of every watched channel, one upstream download serves all viewers
  of a channel (98 % hit rate). Size it as channels x segment size x 6.
- Goroutines return to the idle count after the run (no leak).

Micro benchmarks (`go test -bench`), cached segment: 5.7 us/op, 775 B/op, 14 allocs/op (was 9.4 us, 29 allocs before
pooling the HMAC and building the canonical string without copies); cached playlist: 4.3 us/op, 11 allocs/op.
