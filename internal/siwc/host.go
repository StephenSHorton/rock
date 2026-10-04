package siwc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type hostFile struct {
	ExtAgentHostID string `json:"ext_agent_host_id"`
}

// HostID returns this install's stable ext_agent_host_id, creating a
// urn:uuid: value on first use. The same host reuses the same id.
func HostID() (string, error) {
	path := hostPath()
	if rejectsCodexPath(path) {
		return "", fmt.Errorf("siwc: refusing a Codex host path")
	}
	if raw, err := os.ReadFile(path); err == nil {
		var hf hostFile
		if err := json.Unmarshal(raw, &hf); err == nil && hf.ExtAgentHostID != "" {
			return hf.ExtAgentHostID, nil
		}
	}
	id, err := newHostID()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(hostFile{ExtAgentHostID: id}, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Chmod(path, 0o600)
	return id, nil
}

func newHostID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
