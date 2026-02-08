package homer

import "testing"

func TestExtractSDPMedia(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "INVITE with PCMA codec",
			raw: "INVITE sip:123@10.0.0.1 SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 10.0.0.2\r\n" +
				"Content-Type: application/sdp\r\n" +
				"\r\n" +
				"v=0\r\n" +
				"o=root 123 456 IN IP4 10.0.0.2\r\n" +
				"s=call\r\n" +
				"c=IN IP4 10.0.0.2\r\n" +
				"m=audio 17818 RTP/AVP 8 0 101\r\n" +
				"a=rtpmap:8 PCMA/8000\r\n" +
				"a=rtpmap:0 PCMU/8000\r\n" +
				"a=rtpmap:101 telephone-event/8000\r\n",
			want: "PCMA :17818",
		},
		{
			name: "200 OK with G729 codec",
			raw: "SIP/2.0 200 OK\r\n" +
				"Via: SIP/2.0/UDP 10.0.0.1\r\n" +
				"\r\n" +
				"v=0\r\n" +
				"m=audio 20000 RTP/AVP 18\r\n" +
				"a=rtpmap:18 G729/8000\r\n",
			want: "G729 :20000",
		},
		{
			name: "no SDP body",
			raw: "BYE sip:123@10.0.0.1 SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 10.0.0.2\r\n",
			want: "",
		},
		{
			name: "SDP without m=audio",
			raw: "INVITE sip:123@10.0.0.1 SIP/2.0\r\n" +
				"\r\n" +
				"v=0\r\n" +
				"o=root 123 456 IN IP4 10.0.0.2\r\n",
			want: "",
		},
		{
			name: "m=audio without rtpmap",
			raw: "INVITE sip:123@10.0.0.1 SIP/2.0\r\n" +
				"\r\n" +
				"v=0\r\n" +
				"m=audio 5060 RTP/AVP 8\r\n",
			want: ":5060",
		},
		{
			name: "empty raw",
			raw:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSDPMedia(tt.raw)
			if got != tt.want {
				t.Errorf("ExtractSDPMedia() = %q, want %q", got, tt.want)
			}
		})
	}
}
