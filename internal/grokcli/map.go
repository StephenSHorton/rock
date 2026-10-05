package grokcli

import "strings"

// DeriveGrokTiers picks fast/strong ids from the child's advertised list.
// Names come only from availableModels — Rock never invents model ids.
func DeriveGrokTiers(available []ModelInfo, current string) (fast, strong string) {
	for _, m := range available {
		if isFastModel(m) {
			if fast == "" {
				fast = m.ID
			}
		}
	}
	for _, m := range available {
		if m.ID == current && !isFastModel(m) {
			strong = m.ID
			break
		}
	}
	if strong == "" {
		for _, m := range available {
			if !isFastModel(m) {
				strong = m.ID
				break
			}
		}
	}
	if strong == "" {
		strong = strings.TrimSpace(current)
	}
	if strong == "" && len(available) > 0 {
		strong = available[0].ID
	}
	if fast == "" {
		fast = strings.TrimSpace(current)
	}
	if fast == "" {
		fast = strong
	}
	return fast, strong
}

func isFastModel(m ModelInfo) bool {
	id := strings.ToLower(m.ID)
	name := strings.ToLower(m.Name)
	return strings.HasSuffix(id, "-fast") ||
		strings.Contains(id, "build-fast") ||
		strings.Contains(name, "fast")
}

// MapGrokTarget resolves a harness model argument (OpenAI name, Jev tier
// label, or grok id) onto a child-advertised id. override (config
// grok_model) wins when it is in available. OpenAI names and unknown
// ids never become -m flags.
func MapGrokTarget(want string, available []ModelInfo, current, override string) string {
	override = strings.TrimSpace(override)
	if override != "" && modelInList(override, available) {
		return override
	}
	if override != "" && len(available) == 0 {
		// Child has not listed models yet; still honor an explicit pick
		// only when it does not look like an OpenAI/tier label.
		if classifyTier(override) == "" {
			return override
		}
	}
	want = strings.TrimSpace(want)
	if want != "" && modelInList(want, available) {
		return want
	}
	fast, strong := DeriveGrokTiers(available, current)
	switch classifyTier(want) {
	case "strong":
		if strong != "" {
			return strong
		}
	case "fast":
		if fast != "" {
			return fast
		}
	}
	if current != "" {
		return current
	}
	if fast != "" {
		return fast
	}
	return strong
}

func classifyTier(want string) string {
	w := strings.ToLower(strings.TrimSpace(want))
	switch w {
	case "", "default", "auto":
		return ""
	case "strong":
		return "strong"
	case "fast":
		return "fast"
	}
	if strings.HasPrefix(w, "gpt-") || strings.HasPrefix(w, "o1") || strings.HasPrefix(w, "o3") || strings.HasPrefix(w, "chatgpt") {
		if strings.Contains(w, "mini") || strings.Contains(w, "nano") || strings.Contains(w, "3.5") {
			return "fast"
		}
		return "strong"
	}
	return ""
}

func modelInList(id string, available []ModelInfo) bool {
	for _, m := range available {
		if m.ID == id {
			return true
		}
	}
	return false
}

// LooksLikeForeignModel reports OpenAI / tier labels that must never be
// passed to grok as -m.
func LooksLikeForeignModel(id string) bool {
	return classifyTier(id) != "" || strings.EqualFold(strings.TrimSpace(id), "fast") || strings.EqualFold(strings.TrimSpace(id), "strong")
}
