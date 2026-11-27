package usershelper

import (
	"fmt"
	"strconv"

	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/settings/internal/pwfiles"
)

// start is the first of the rule's IDs of s the new account of uid
// takes, given the accounts' UID_MIN: as useradd finds it, the lowest
// free one or the deterministic one.
func (r subIDRule) start(db pwfiles.DB, s pwfiles.SubIDs, uidMin, uid uint32) (uint32, error) {
	if !r.deterministic {
		return db.FreeSubIDs(s, r.ids, r.count)
	}
	name := s.File().Name()
	if uid < uidMin {
		return 0, fmt.Errorf("/etc/%s: uid %d lies below UID_MIN %d, so it has no deterministic range", name, uid, uidMin)
	}
	lo, hi, count := uint64(r.ids.Min), uint64(r.ids.Max), uint64(r.count)
	if lo > hi || count == 0 || count > hi-lo+1 {
		return 0, fmt.Errorf("/etc/%s: %d-%d holds no range of %d IDs", name, r.ids.Min, r.ids.Max, r.count)
	}
	slots, slot := (hi-lo+1)/count, uint64(uid-uidMin)
	if slot >= slots {
		if !r.wrap {
			return 0, fmt.Errorf("/etc/%s: uid %d is past the %d ranges of %d IDs in %d-%d: %w", name, uid, slots, r.count, r.ids.Min, r.ids.Max, pwfiles.ErrExhausted)
		}
		slot %= slots
	}
	return intconv.Must[uint32](lo + slot*count), nil
}

// addSubIDs gives the new account login of uid its subordinate IDs as
// useradd does: of each kind whose file exists, that no NSS module
// keeps and whose COUNT is not 0, to an account of a uid in the range
// of people's that holds none yet. NixOS rewrites subuid and subgid
// from its configuration on every activation, so there it adds none.
func (s system) addSubIDs(files *pwfiles.Files, defs defaults, login string, uid uint32) error {
	if s.nixos != nil || defs.subIDModule != "" || uid == 0 || uid < defs.uids.Min || uid > defs.uids.Max {
		return nil
	}
	for _, ids := range []pwfiles.SubIDs{pwfiles.SubUIDs, pwfiles.SubGIDs} {
		rule := defs.subIDs[ids]
		if rule.count == 0 || !files.Exists(ids.File()) || files.HasSubIDs(ids, login, uid) {
			continue
		}
		start, err := rule.start(files.DB, ids, defs.uids.Min, uid)
		if err != nil {
			return fmt.Errorf("give %s subordinate IDs: %w", login, err)
		}
		owner := login
		if rule.storeByUID {
			owner = strconv.FormatUint(uint64(uid), 10)
		}
		if err := files.AddSubIDs(ids, owner, start, rule.count); err != nil {
			return err
		}
	}
	return nil
}
