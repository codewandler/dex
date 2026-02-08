package homer

import (
	"strings"
)

// ExtractSIPHeader finds the value of a SIP header in a raw message body.
// Returns "" if not found. Header matching is case-insensitive.
// Stops at the first empty line (end of headers / start of SDP body).
func ExtractSIPHeader(raw string, header string) string {
	headerLower := strings.ToLower(header) + ":"
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break // end of SIP headers
		}
		if strings.HasPrefix(strings.ToLower(line), headerLower) {
			return strings.TrimSpace(line[len(headerLower):])
		}
	}
	return ""
}

// ExtractSIPHeadersByPrefix returns all headers whose name starts with prefix (case-insensitive).
// Returns map[canonicalHeaderName]value. Stops at empty line.
func ExtractSIPHeadersByPrefix(raw string, prefix string) map[string]string {
	result := make(map[string]string)
	prefixLower := strings.ToLower(prefix)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break
		}
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		name := line[:colonIdx]
		if strings.HasPrefix(strings.ToLower(name), prefixLower) {
			result[name] = strings.TrimSpace(line[colonIdx+1:])
		}
	}
	return result
}

// ExtractSIPAllHeaders returns all SIP headers as map[canonicalName]value.
// Stops at empty line. Skips the request/status line (first line).
func ExtractSIPAllHeaders(raw string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if i == 0 {
			continue // skip request/status line
		}
		if line == "" {
			break
		}
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			continue
		}
		name := line[:colonIdx]
		result[name] = strings.TrimSpace(line[colonIdx+1:])
	}
	return result
}

// ExtractSDP returns the SDP body (everything after the first blank line), or "".
func ExtractSDP(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" && i > 0 {
			rest := strings.Join(lines[i+1:], "\n")
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// ExtractSDPMedia extracts a compact media description from an SDP body embedded in a raw SIP message.
// Returns e.g. "PCMA :17818" (codec + port) or "" if no SDP or no audio media line.
func ExtractSDPMedia(raw string) string {
	sdp := ExtractSDP(raw)
	if sdp == "" {
		return ""
	}

	var port string
	var codec string
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimRight(line, "\r")
		// m=audio 17818 RTP/AVP 8 0 101
		if strings.HasPrefix(line, "m=audio ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				port = parts[1]
			}
		}
		// a=rtpmap:8 PCMA/8000
		if codec == "" && strings.HasPrefix(line, "a=rtpmap:") {
			// value is "8 PCMA/8000" — take the codec name before the slash
			val := line[len("a=rtpmap:"):]
			parts := strings.Fields(val)
			if len(parts) >= 2 {
				name := parts[1]
				if slashIdx := strings.IndexByte(name, '/'); slashIdx >= 0 {
					name = name[:slashIdx]
				}
				codec = name
			}
		}
	}

	if port == "" {
		return ""
	}
	if codec != "" {
		return codec + " :" + port
	}
	return ":" + port
}
