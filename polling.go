package pn532

// tagState holds cached tag detection data, written by polling callbacks
// under s.mu.Lock() and read by Readings() under s.mu.RLock().
type tagState struct {
	deviceHealthy   bool
	tagPresent      bool
	uid             string
	tagType         string
	manufacturer    string
	isGenuine       bool
	ndefText        string
	ndefRecordCount int
	ntagVariant     string
	mifareVariant   string
	userMemoryBytes int
}

func buildReadingsFromState(state *tagState) map[string]interface{} {
	if !state.deviceHealthy {
		return map[string]interface{}{
			"status":         "connected",
			"device_healthy": false,
			"tag_present":    false,
		}
	}

	if !state.tagPresent {
		return map[string]interface{}{
			"status":         "connected",
			"device_healthy": true,
			"tag_present":    false,
		}
	}

	return map[string]interface{}{
		"status":           "connected",
		"device_healthy":   true,
		"tag_present":      true,
		"uid":              state.uid,
		"tag_type":         state.tagType,
		"manufacturer":     state.manufacturer,
		"is_genuine":       state.isGenuine,
		"ntag_variant":     state.ntagVariant,
		"mifare_variant":   state.mifareVariant,
		"user_memory_bytes": state.userMemoryBytes,
		"ndef_text":        state.ndefText,
		"ndef_record_count": state.ndefRecordCount,
	}
}
