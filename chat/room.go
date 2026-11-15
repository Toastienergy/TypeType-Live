package chat

import "strconv"

type RoomState struct {
	FollowersOnly int  `json:"followersOnly"`
	Slow          int  `json:"slow"`
	EmoteOnly     bool `json:"emoteOnly"`
	SubsOnly      bool `json:"subsOnly"`
	UniqueChat    bool `json:"uniqueChat"`
}

func newRoomState() RoomState {
	return RoomState{FollowersOnly: -1}
}

func (s RoomState) apply(tags map[string]string) RoomState {
	if v, ok := tags["followers-only"]; ok {
		s.FollowersOnly = atoiOr(v, -1)
	}
	if v, ok := tags["slow"]; ok {
		s.Slow = atoiOr(v, 0)
	}
	if v, ok := tags["emote-only"]; ok {
		s.EmoteOnly = v == "1"
	}
	if v, ok := tags["subs-only"]; ok {
		s.SubsOnly = v == "1"
	}
	if v, ok := tags["r9k"]; ok {
		s.UniqueChat = v == "1"
	}
	return s
}

func atoiOr(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
