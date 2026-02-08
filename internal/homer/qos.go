package homer

import (
	"encoding/json"
	"math"
	"sort"
	"time"
)

// QoSResult holds the response from a QoS report query
type QoSResult struct {
	RTCP QoSData `json:"rtcp"`
	RTP  QoSData `json:"rtp"`
}

// QoSData holds a list of QoS reports
type QoSData struct {
	Data  []QoSReport `json:"data"`
	Total int         `json:"total"`
}

// QoSReport represents a single RTCP/RTP report from Homer.
// The transport metadata (IPs, ports) is at the top level.
// The RTCP message data is in the Raw field as a JSON string that must be
// decoded separately via ParseMessage().
type QoSReport struct {
	ID            int             `json:"id"`
	SrcIP         string          `json:"srcIp"`
	SrcPort       int             `json:"srcPort"`
	DstIP         string          `json:"dstIp"`
	DstPort       int             `json:"dstPort"`
	CallID        string          `json:"sid"`
	CorrelationID string          `json:"correlation_id"`
	CreateDate    string          `json:"create_date"`
	Proto         string          `json:"proto"`
	TimeSeconds   int64           `json:"timeSeconds"`
	TimeUseconds  int64           `json:"timeUseconds"`
	Raw           string          `json:"raw"`
	Node          json.RawMessage `json:"node"`
}

// CreateTime parses the create_date string into a time.Time.
func (r QoSReport) CreateTime() time.Time {
	// Try RFC3339 variants (Homer uses "2006-01-02T15:04:05.999999Z")
	for _, fmt := range []string{
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339Nano,
		time.RFC3339,
	} {
		if t, err := time.Parse(fmt, r.CreateDate); err == nil {
			return t
		}
	}
	// Fallback to epoch seconds
	if r.TimeSeconds > 0 {
		return time.Unix(r.TimeSeconds, r.TimeUseconds*1000)
	}
	return time.Time{}
}

// ParseMessage decodes the Raw JSON string into an RTCPMessage.
func (r QoSReport) ParseMessage() (RTCPMessage, error) {
	var msg RTCPMessage
	if r.Raw == "" {
		return msg, nil
	}
	err := json.Unmarshal([]byte(r.Raw), &msg)
	return msg, err
}

// RTCPMessage holds decoded RTCP message fields from the raw JSON string.
type RTCPMessage struct {
	Type              int               `json:"type"` // 200=SR, 201=RR, 202=SDES
	SSRC              uint32            `json:"ssrc"`
	ReportCount       int               `json:"report_count"`
	SenderInformation SenderInformation `json:"sender_information"`
	ReportBlocks      []ReportBlock     `json:"report_blocks"`
}

// SenderInformation holds sender report statistics
type SenderInformation struct {
	Packets         uint32 `json:"packets"`
	Octets          uint32 `json:"octets"`
	NTPTimestampSec uint32 `json:"ntp_timestamp_sec"`
	NTPTimestampUSec uint32 `json:"ntp_timestamp_usec"`
	RTPTimestamp    uint32 `json:"rtp_timestamp"`
}

// ReportBlock holds a single RTCP report block
type ReportBlock struct {
	SourceSSRC   uint32  `json:"source_ssrc"`
	FractionLost float64 `json:"fraction_lost"`
	PacketsLost  int     `json:"packets_lost"`
	HighestSeqNo uint32  `json:"highest_seq_no"`
	IAJitter     uint32  `json:"ia_jitter"`
	LSR          uint32  `json:"lsr"`
	DLSR         uint32  `json:"dlsr"`
}

// StreamKey identifies a unique media stream by its endpoints
type StreamKey struct {
	SrcIP   string
	SrcPort int
	DstIP   string
	DstPort int
}

// StreamMetrics holds aggregated quality metrics for a single media stream
type StreamMetrics struct {
	SrcIP       string    `json:"src_ip"`
	SrcPort     int       `json:"src_port"`
	DstIP       string    `json:"dst_ip"`
	DstPort     int       `json:"dst_port"`
	Reports     int       `json:"reports"`
	Packets     uint32    `json:"packets"`
	PacketsLost int       `json:"packets_lost"`
	LossPercent float64   `json:"loss_percent"`
	AvgJitterMS float64   `json:"avg_jitter_ms"`
	MaxJitterMS float64   `json:"max_jitter_ms"`
	MOS         float64   `json:"mos"`
	FirstReport time.Time `json:"first_report"`
	LastReport  time.Time `json:"last_report"`
}

// AggregateStreams groups RTCP reports by media stream and computes per-stream metrics.
// clockRate is the RTP clock rate in Hz (typically 8000 for G.711).
// defaultLatencyMS is the assumed one-way network latency in ms for MOS calculation.
func AggregateStreams(qos *QoSResult, clockRate int, defaultLatencyMS float64) []StreamMetrics {
	if clockRate <= 0 {
		clockRate = 8000
	}

	type accumulator struct {
		key         StreamKey
		reports     int
		packets     uint32
		packetsLost int
		totalJitter float64
		maxJitter   float64
		firstTS     time.Time
		lastTS      time.Time
	}

	streams := make(map[StreamKey]*accumulator)

	process := func(reports []QoSReport) {
		for _, r := range reports {
			msg, err := r.ParseMessage()
			if err != nil || len(msg.ReportBlocks) == 0 {
				continue
			}

			key := StreamKey{
				SrcIP:   r.SrcIP,
				SrcPort: r.SrcPort,
				DstIP:   r.DstIP,
				DstPort: r.DstPort,
			}

			ts := r.CreateTime()

			acc, ok := streams[key]
			if !ok {
				acc = &accumulator{key: key, firstTS: ts, lastTS: ts}
				streams[key] = acc
			}

			if ts.Before(acc.firstTS) {
				acc.firstTS = ts
			}
			if ts.After(acc.lastTS) {
				acc.lastTS = ts
			}

			// Sender report: accumulate packet count
			if msg.SenderInformation.Packets > 0 {
				acc.packets += msg.SenderInformation.Packets
			}

			for _, rb := range msg.ReportBlocks {
				acc.reports++
				acc.packetsLost += rb.PacketsLost

				// Convert jitter from timestamp units to milliseconds
				jitterMS := float64(rb.IAJitter) / float64(clockRate) * 1000
				acc.totalJitter += jitterMS
				if jitterMS > acc.maxJitter {
					acc.maxJitter = jitterMS
				}
			}
		}
	}

	process(qos.RTCP.Data)
	process(qos.RTP.Data)

	result := make([]StreamMetrics, 0, len(streams))
	for _, acc := range streams {
		if acc.reports == 0 {
			continue
		}

		avgJitter := acc.totalJitter / float64(acc.reports)

		var lossPct float64
		if acc.packets > 0 {
			lossPct = float64(acc.packetsLost) / float64(acc.packets) * 100
			if lossPct < 0 {
				lossPct = 0
			}
		}

		mos := CalculateMOS(defaultLatencyMS, avgJitter, lossPct)

		result = append(result, StreamMetrics{
			SrcIP:       acc.key.SrcIP,
			SrcPort:     acc.key.SrcPort,
			DstIP:       acc.key.DstIP,
			DstPort:     acc.key.DstPort,
			Reports:     acc.reports,
			Packets:     acc.packets,
			PacketsLost: acc.packetsLost,
			LossPercent: lossPct,
			AvgJitterMS: avgJitter,
			MaxJitterMS: acc.maxJitter,
			MOS:         mos,
			FirstReport: acc.firstTS,
			LastReport:  acc.lastTS,
		})
	}

	// Sort by first report time
	sort.Slice(result, func(i, j int) bool {
		return result[i].FirstReport.Before(result[j].FirstReport)
	})

	return result
}

// CalculateMOS computes a Mean Opinion Score estimate using the E-model approximation.
// Parameters: one-way latency (ms), jitter (ms), packet loss (%).
// Returns a value clamped to [1.0, 4.5].
func CalculateMOS(latencyMS, jitterMS, lossPercent float64) float64 {
	// Effective latency includes jitter buffer delay (2x jitter) plus codec delay (10ms)
	effectiveLatency := latencyMS + jitterMS*2 + 10

	// R-factor base
	R := 93.2 - effectiveLatency/40

	// Loss impairment
	if lossPercent > 0 {
		R -= 2.5*lossPercent + 0.03*lossPercent*lossPercent
	}

	// Clamp R to valid range
	if R < 0 {
		R = 0
	}
	if R > 100 {
		R = 100
	}

	// Convert R to MOS
	mos := 1 + 0.035*R + R*(R-60)*(100-R)*7e-6

	// Clamp MOS
	mos = math.Round(mos*100) / 100
	if mos < 1.0 {
		mos = 1.0
	}
	if mos > 4.5 {
		mos = 4.5
	}

	return mos
}
