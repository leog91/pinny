package capture

// Plan filters receipts and IDs already in notes or Trash. Remember receipts even
// for existing notes so permanent local deletion never recreates an acknowledged
// delivery after a lost acknowledgment. The caller saves the plan transactionally.
func Plan(endpoint string, items []Item, receipts map[string]bool, existing map[string]bool) (fresh []Item, next map[string]bool) {
	next = make(map[string]bool, len(receipts)+len(items))
	for id, received := range receipts {
		next[id] = received
	}
	for _, item := range items {
		id := NoteID(endpoint, item.ID)
		if !next[id] && !existing[id] {
			fresh = append(fresh, item)
		}
		next[id] = true
	}
	return
}
