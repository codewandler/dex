package homer

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCalculateMOS(t *testing.T) {
	tests := []struct {
		name      string
		latencyMS float64
		jitterMS  float64
		lossPct   float64
		wantMin   float64
		wantMax   float64
	}{
		{
			name:      "perfect conditions",
			latencyMS: 20, jitterMS: 0, lossPct: 0,
			wantMin: 4.3, wantMax: 4.5,
		},
		{
			name:      "moderate loss",
			latencyMS: 20, jitterMS: 5, lossPct: 3,
			wantMin: 3.6, wantMax: 4.3,
		},
		{
			name:      "heavy loss",
			latencyMS: 200, jitterMS: 50, lossPct: 20,
			wantMin: 1.0, wantMax: 2.0,
		},
		{
			name:      "zero latency zero loss",
			latencyMS: 0, jitterMS: 0, lossPct: 0,
			wantMin: 4.3, wantMax: 4.5,
		},
		{
			name:      "extreme values clamp to minimum",
			latencyMS: 1000, jitterMS: 500, lossPct: 100,
			wantMin: 1.0, wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateMOS(tt.latencyMS, tt.jitterMS, tt.lossPct)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateMOS(%v, %v, %v) = %v, want [%v, %v]",
					tt.latencyMS, tt.jitterMS, tt.lossPct, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCalculateMOS_Clamping(t *testing.T) {
	mos := CalculateMOS(0, 0, 0)
	if mos > 4.5 {
		t.Errorf("MOS %v exceeds 4.5 ceiling", mos)
	}
	if mos < 1.0 {
		t.Errorf("MOS %v below 1.0 floor", mos)
	}
}

func makeRaw(t *testing.T, msg RTCPMessage) string {
	t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAggregateStreams(t *testing.T) {
	t.Run("bidirectional streams", func(t *testing.T) {
		qos := &QoSResult{
			RTCP: QoSData{
				Data: []QoSReport{
					{
						SrcIP: "192.0.2.1", SrcPort: 54835,
						DstIP: "192.0.2.2", DstPort: 58445,
						CreateDate: "2026-01-01T00:00:01.000000Z",
						Raw: makeRaw(t, RTCPMessage{
							SenderInformation: SenderInformation{Packets: 100},
							ReportBlocks: []ReportBlock{
								{IAJitter: 80, PacketsLost: 0},
							},
						}),
					},
					{
						SrcIP: "192.0.2.2", SrcPort: 58445,
						DstIP: "192.0.2.1", DstPort: 54835,
						CreateDate: "2026-01-01T00:00:02.000000Z",
						Raw: makeRaw(t, RTCPMessage{
							SenderInformation: SenderInformation{Packets: 100},
							ReportBlocks: []ReportBlock{
								{IAJitter: 160, PacketsLost: 2},
							},
						}),
					},
				},
			},
		}

		streams := AggregateStreams(qos, 8000, 20)
		if len(streams) != 2 {
			t.Fatalf("expected 2 streams, got %d", len(streams))
		}

		// First stream: 192.0.2.1 → 192.0.2.2 (jitter=80/8000*1000 = 10ms)
		s1 := streams[0]
		if s1.SrcIP != "192.0.2.1" || s1.DstIP != "192.0.2.2" {
			t.Errorf("stream 1: unexpected endpoints %s:%d → %s:%d", s1.SrcIP, s1.SrcPort, s1.DstIP, s1.DstPort)
		}
		if s1.Reports != 1 {
			t.Errorf("stream 1: expected 1 report, got %d", s1.Reports)
		}
		if s1.PacketsLost != 0 {
			t.Errorf("stream 1: expected 0 packets lost, got %d", s1.PacketsLost)
		}
		expectedJitter := 80.0 / 8000.0 * 1000.0 // 10ms
		if math.Abs(s1.AvgJitterMS-expectedJitter) > 0.01 {
			t.Errorf("stream 1: expected avg jitter %.2f, got %.2f", expectedJitter, s1.AvgJitterMS)
		}

		// Second stream: 192.0.2.2 → 192.0.2.1 (jitter=160/8000*1000 = 20ms, loss=2)
		s2 := streams[1]
		if s2.PacketsLost != 2 {
			t.Errorf("stream 2: expected 2 packets lost, got %d", s2.PacketsLost)
		}
		expectedJitter2 := 160.0 / 8000.0 * 1000.0 // 20ms
		if math.Abs(s2.AvgJitterMS-expectedJitter2) > 0.01 {
			t.Errorf("stream 2: expected avg jitter %.2f, got %.2f", expectedJitter2, s2.AvgJitterMS)
		}
	})

	t.Run("different clock rate", func(t *testing.T) {
		qos := &QoSResult{
			RTCP: QoSData{
				Data: []QoSReport{
					{
						SrcIP: "10.0.0.1", SrcPort: 5000,
						DstIP: "10.0.0.2", DstPort: 6000,
						CreateDate: "2026-01-01T00:00:01.000000Z",
						Raw: makeRaw(t, RTCPMessage{
							SenderInformation: SenderInformation{Packets: 50},
							ReportBlocks: []ReportBlock{
								{IAJitter: 160, PacketsLost: 0},
							},
						}),
					},
				},
			},
		}

		// At 16000 Hz, jitter 160 = 160/16000*1000 = 10ms
		streams := AggregateStreams(qos, 16000, 20)
		if len(streams) != 1 {
			t.Fatalf("expected 1 stream, got %d", len(streams))
		}
		expectedJitter := 160.0 / 16000.0 * 1000.0 // 10ms
		if math.Abs(streams[0].AvgJitterMS-expectedJitter) > 0.01 {
			t.Errorf("expected jitter %.2f ms, got %.2f ms", expectedJitter, streams[0].AvgJitterMS)
		}
	})

	t.Run("zero reports", func(t *testing.T) {
		qos := &QoSResult{}
		streams := AggregateStreams(qos, 8000, 20)
		if len(streams) != 0 {
			t.Errorf("expected 0 streams, got %d", len(streams))
		}
	})

	t.Run("report without report blocks", func(t *testing.T) {
		qos := &QoSResult{
			RTCP: QoSData{
				Data: []QoSReport{
					{
						SrcIP: "10.0.0.1", SrcPort: 5000,
						DstIP: "10.0.0.2", DstPort: 6000,
						CreateDate: "2026-01-01T00:00:01.000000Z",
						Raw: makeRaw(t, RTCPMessage{
							SenderInformation: SenderInformation{Packets: 50},
						}),
					},
				},
			},
		}

		streams := AggregateStreams(qos, 8000, 20)
		if len(streams) != 0 {
			t.Errorf("expected 0 streams (no report blocks), got %d", len(streams))
		}
	})

	t.Run("zero packets guards against division by zero", func(t *testing.T) {
		qos := &QoSResult{
			RTCP: QoSData{
				Data: []QoSReport{
					{
						SrcIP: "10.0.0.1", SrcPort: 5000,
						DstIP: "10.0.0.2", DstPort: 6000,
						CreateDate: "2026-01-01T00:00:01.000000Z",
						Raw: makeRaw(t, RTCPMessage{
							// No sender info → packets=0
							ReportBlocks: []ReportBlock{
								{IAJitter: 80, PacketsLost: 5},
							},
						}),
					},
				},
			},
		}

		streams := AggregateStreams(qos, 8000, 20)
		if len(streams) != 1 {
			t.Fatalf("expected 1 stream, got %d", len(streams))
		}
		if streams[0].LossPercent != 0 {
			t.Errorf("expected 0%% loss with 0 packets, got %.2f%%", streams[0].LossPercent)
		}
	})
}

func TestQoSReport_CreateTime(t *testing.T) {
	r := QoSReport{CreateDate: "2026-02-07T00:10:14.943753Z"}
	ts := r.CreateTime()
	if ts.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if ts.Year() != 2026 || ts.Month() != 2 || ts.Day() != 7 {
		t.Errorf("unexpected date: %v", ts)
	}
}

func TestQoSReport_ParseMessage(t *testing.T) {
	raw := `{"sender_information":{"ntp_timestamp_sec":3979411814,"ntp_timestamp_usec":4053179190,"rtp_timestamp":18880,"packets":118,"octets":18880},"ssrc":797632257,"type":202,"report_count":1,"report_blocks":[{"source_ssrc":1354749014,"fraction_lost":0,"packets_lost":0,"highest_seq_no":42710,"ia_jitter":1,"lsr":0,"dlsr":2196173160}]}`
	r := QoSReport{Raw: raw}
	msg, err := r.ParseMessage()
	if err != nil {
		t.Fatalf("ParseMessage() error: %v", err)
	}
	if msg.SSRC != 797632257 {
		t.Errorf("expected SSRC 797632257, got %d", msg.SSRC)
	}
	if len(msg.ReportBlocks) != 1 {
		t.Fatalf("expected 1 report block, got %d", len(msg.ReportBlocks))
	}
	if msg.ReportBlocks[0].IAJitter != 1 {
		t.Errorf("expected IAJitter 1, got %d", msg.ReportBlocks[0].IAJitter)
	}
	if msg.SenderInformation.Packets != 118 {
		t.Errorf("expected 118 packets, got %d", msg.SenderInformation.Packets)
	}
}
