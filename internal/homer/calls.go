package homer

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// CallSummary represents a grouped view of a SIP call (all messages sharing one Call-ID)
type CallSummary struct {
	CallID    string        `json:"call_id"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Caller    string        `json:"caller"`
	Callee    string        `json:"callee"`
	Direction string        `json:"direction,omitempty"` // "IN", "OUT", or ""
	Status    string        `json:"status"`              // "answered", "busy", "cancelled", "no answer", "failed", "ringing"
	MsgCount  int           `json:"msg_count"`
	Messages  []CallRecord  `json:"-"`
}

// GroupCalls groups raw SIP messages by Call-ID and produces call summaries.
// If number is non-empty, direction is detected relative to that number.
func GroupCalls(records []CallRecord, number string) []CallSummary {
	// Group by Call-ID
	groups := make(map[string][]CallRecord)
	for _, r := range records {
		groups[r.CallID] = append(groups[r.CallID], r)
	}

	var summaries []CallSummary
	for callID, msgs := range groups {
		// Sort messages by time
		sort.Slice(msgs, func(i, j int) bool {
			return msgs[i].Date < msgs[j].Date
		})

		cs := CallSummary{
			CallID:   callID,
			MsgCount: len(msgs),
			Messages: msgs,
		}

		if len(msgs) > 0 {
			cs.StartTime = time.UnixMilli(msgs[0].Date)
			cs.EndTime = time.UnixMilli(msgs[len(msgs)-1].Date)
			cs.Duration = cs.EndTime.Sub(cs.StartTime)
		}

		// Extract caller/callee from the first INVITE
		for _, m := range msgs {
			if m.Method == "INVITE" {
				cs.Caller = m.FromUser
				cs.Callee = m.ToUser
				if cs.Callee == "" {
					cs.Callee = m.RuriUser
				}
				break
			}
		}

		// Fallback: if no INVITE found, use first message
		if cs.Caller == "" && len(msgs) > 0 {
			cs.Caller = msgs[0].FromUser
			cs.Callee = msgs[0].ToUser
			if cs.Callee == "" {
				cs.Callee = msgs[0].RuriUser
			}
		}

		if number != "" {
			cs.Direction = detectDirection(cs.Caller, cs.Callee, number)
		}

		cs.Status = deriveStatus(msgs)
		summaries = append(summaries, cs)
	}

	// Sort by start time descending (newest first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StartTime.After(summaries[j].StartTime)
	})

	return summaries
}

// FetchCalls discovers calls matching the search parameters. Returns up to maxCalls results.
//
// Uses bounded backward pagination (up to maxBatches × batchLimit messages),
// walking backwards in time to discover unique Call-IDs. Messages are grouped
// by Call-ID to produce call summaries.
func (c *Client) FetchCalls(params SearchParams, number string, maxCalls int) ([]CallSummary, error) {
	const (
		batchLimit = 200 // messages per discovery request (safe for Homer API)
		maxBatches = 5   // max discovery iterations to avoid runaway requests
	)

	// Phase 1: Discover unique Call-IDs via bounded backward pagination.
	seenCallIDs := make(map[string]bool)
	var allDiscovered []CallRecord
	discoverTo := params.To

	for batch := 0; batch < maxBatches; batch++ {
		if !discoverTo.After(params.From) {
			break
		}

		batchParams := params
		batchParams.To = discoverTo
		batchParams.Limit = batchLimit

		result, err := c.SearchCalls(batchParams)
		if err != nil {
			return nil, err
		}
		if len(result.Data) == 0 {
			break
		}

		// Track Call-IDs and find oldest timestamp for pagination.
		var minTS int64
		for i := range result.Data {
			seenCallIDs[result.Data[i].CallID] = true
			if minTS == 0 || result.Data[i].Date < minTS {
				minTS = result.Data[i].Date
			}
		}
		allDiscovered = append(allDiscovered, result.Data...)

		// Stop if batch was not full (no more data) or we have enough Call-IDs.
		if len(result.Data) < batchLimit || len(seenCallIDs) >= maxCalls {
			break
		}

		// Advance window to before the oldest message we received.
		discoverTo = time.UnixMilli(minTS).Add(-time.Millisecond)
	}

	if len(allDiscovered) == 0 {
		return nil, nil
	}

	calls := GroupCalls(allDiscovered, number)
	if len(calls) > maxCalls {
		calls = calls[:maxCalls]
	}
	return calls, nil
}

// MergeSearchResults deduplicates two search results by message ID.
func MergeSearchResults(a, b *SearchResult) *SearchResult {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	seen := make(map[float64]bool, len(a.Data))
	for _, r := range a.Data {
		seen[r.ID] = true
	}

	merged := make([]CallRecord, len(a.Data))
	copy(merged, a.Data)

	for _, r := range b.Data {
		if !seen[r.ID] {
			merged = append(merged, r)
			seen[r.ID] = true
		}
	}

	return &SearchResult{Data: merged}
}

// DeriveRoute extracts unique IP hop pairs from a call's messages.
// Returns pairs of [src, dst] IPs in the order they first appear.
func DeriveRoute(msgs []CallRecord) [][2]string {
	var pairs [][2]string
	seen := make(map[[2]string]bool)
	for _, m := range msgs {
		pair := [2]string{m.SourceIP, m.DestIP}
		if !seen[pair] {
			seen[pair] = true
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

// FormatRoute formats IP hop pairs into a compact chain like "A → B → C".
// Collapses consecutive hops that share an IP endpoint.
func FormatRoute(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}

	// Build chain by collapsing shared IPs
	chain := []string{pairs[0][0], pairs[0][1]}
	for i := 1; i < len(pairs); i++ {
		if pairs[i][0] == chain[len(chain)-1] {
			chain = append(chain, pairs[i][1])
		} else {
			// Discontinuous hop — append both
			chain = append(chain, pairs[i][0], pairs[i][1])
		}
	}

	return strings.Join(chain, " → ")
}

// detectDirection determines call direction relative to the given number.
// If number matches caller → OUT, matches callee → IN.
func detectDirection(caller, callee, number string) string {
	norm := strings.TrimPrefix(number, "+")
	normCaller := strings.TrimPrefix(caller, "+")
	normCallee := strings.TrimPrefix(callee, "+")

	if strings.Contains(normCaller, norm) || strings.Contains(norm, normCaller) {
		return "OUT"
	}
	if strings.Contains(normCallee, norm) || strings.Contains(norm, normCallee) {
		return "IN"
	}
	return ""
}

// deriveStatus checks SIP response codes to determine call outcome.
// Response messages have numeric strings ("200", "486") in the Method field.
func deriveStatus(msgs []CallRecord) string {
	var highestResponse int

	for _, m := range msgs {
		// Check Method field for numeric response codes
		if code, err := strconv.Atoi(m.Method); err == nil && code >= 100 {
			if code > highestResponse {
				highestResponse = code
			}
		}
		// Also check Status field
		if statusCode := int(m.Status); statusCode >= 100 && statusCode > highestResponse {
			highestResponse = statusCode
		}
	}

	switch {
	case highestResponse >= 200 && highestResponse < 300:
		return "answered"
	case highestResponse == 486:
		return "busy"
	case highestResponse == 487:
		return "cancelled"
	case highestResponse == 408 || highestResponse == 480:
		return "no answer"
	case highestResponse >= 400:
		return "failed"
	case highestResponse >= 100:
		return "ringing"
	default:
		return ""
	}
}
