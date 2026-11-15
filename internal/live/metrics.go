package live

import "sync/atomic"

type Metrics struct {
	MasterFetches   atomic.Uint64
	PlaylistFetches atomic.Uint64
	PlaylistServed  atomic.Uint64
	SegmentFetches  atomic.Uint64
	SegmentServed   atomic.Uint64
	SegmentBytes    atomic.Uint64
	AdPlaylists     atomic.Uint64
	AdSwaps         atomic.Uint64
	AdStripped      atomic.Uint64
	UpstreamErrors  atomic.Uint64
	RestrictedVODs  atomic.Uint64
}
