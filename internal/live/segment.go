package live

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

func (s *Service) ServeSegment(w http.ResponseWriter, r *http.Request, values url.Values) error {
	upstream, err := s.sealer.Open(values.Get("x"))
	if err != nil {
		return err
	}
	if values.Get("m") != "1" {
		s.metrics.SegmentServed.Add(1)
		return s.relay.Stream(w, r, upstream)
	}
	data, ok := s.segments.Get(upstream)
	if !ok {
		data, err = s.segmentGroup.Do(r.Context(), upstream, func(ctx context.Context) ([]byte, error) {
			s.metrics.SegmentFetches.Add(1)
			body, err := s.relay.Fetch(ctx, upstream, maxSegmentBytes)
			if err != nil {
				s.metrics.UpstreamErrors.Add(1)
				return nil, err
			}
			s.segments.Set(upstream, body, liveSegmentCache)
			return body, nil
		})
		if err != nil {
			return err
		}
	}
	s.metrics.SegmentServed.Add(1)
	s.metrics.SegmentBytes.Add(uint64(len(data)))
	w.Header().Set("Content-Type", segmentType(data))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, max-age=30")
	_, err = w.Write(data)
	return err
}

func segmentType(data []byte) string {
	if len(data) > 0 && data[0] == 0x47 {
		return "video/mp2t"
	}
	return "video/mp4"
}
