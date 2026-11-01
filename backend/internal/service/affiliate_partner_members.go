package service

// CUSTOM(fork): 区域归属只由自身锚点和绑定快照决定，不沿实时邀请链扩散。
func ResolvePartnerMembers(partners map[int64]Partner, attachments map[int64]PartnerAttachment, snapshots map[int64]PartnerSnapshot) map[int64]PartnerMember {
	anchor := func(id int64) (PartnerMember, bool) {
		if p, ok := partners[id]; ok {
			return PartnerMember{PartnerID: id, StartAt: p.CreatedAt}, true
		}
		if a, ok := attachments[id]; ok {
			return PartnerMember{PartnerID: a.PartnerID, LineAgentID: id, StartAt: a.AttachedAt}, true
		}
		return PartnerMember{}, false
	}
	out := make(map[int64]PartnerMember)
	for id, s := range snapshots {
		if m, ok := anchor(id); ok {
			out[id] = m
			continue
		}
		for _, target := range []*int64{s.OwnerID, s.UplineID} {
			if target == nil {
				continue
			}
			m, ok := anchor(*target)
			if !ok {
				continue
			}
			bound := s.CreatedAt
			if s.BoundAt != nil {
				bound = *s.BoundAt
			}
			if bound.After(m.StartAt) {
				m.StartAt = bound
			}
			out[id] = m
			break
		}
	}
	return out
}
