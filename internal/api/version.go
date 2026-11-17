package api

import "net/http"

// Build is what the release pipeline stamps into the binary.
type Build struct {
	Version   string
	Revision  string
	BuildTime string
}

type versionBody struct {
	Service       string `json:"service"`
	Version       string `json:"version"`
	Revision      string `json:"revision"`
	ShortRevision string `json:"shortRevision"`
	BuildTime     string `json:"buildTime"`
}

// The about page of the web lists every container with this shape, as the server and token services answer it.
func (b Build) body() versionBody {
	out := versionBody{
		Service: "live", Version: or(b.Version, "dev"), Revision: or(b.Revision, "unknown"),
		BuildTime: or(b.BuildTime, "unknown"),
	}
	out.ShortRevision = out.Revision
	if len(out.ShortRevision) > 12 {
		out.ShortRevision = out.ShortRevision[:12]
	}
	return out
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	writeJSON(w, http.StatusOK, s.build.body())
}
