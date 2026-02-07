package homer

import (
	"strings"
	"testing"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "simple from_user",
			input: "from_user = '999%'",
			want:  "data_header.from_user = '999%'",
		},
		{
			name:  "alias ua",
			input: "ua = 'Asterisk%'",
			want:  "data_header.user_agent = 'Asterisk%'",
		},
		{
			name:  "full name user_agent",
			input: "user_agent = 'FPBX%'",
			want:  "data_header.user_agent = 'FPBX%'",
		},
		{
			name:  "AND",
			input: "from_user = '999%' AND to_user = '1234'",
			want:  "data_header.from_user = '999%' AND data_header.to_user = '1234'",
		},
		{
			name:  "OR",
			input: "from_user = '123' OR to_user = '123'",
			want:  "data_header.from_user = '123' OR data_header.to_user = '123'",
		},
		{
			name:  "mixed AND with parenthesized OR",
			input: "from_user = '999%' AND (to_user = '123' OR to_user = '456')",
			want:  "data_header.from_user = '999%' AND (data_header.to_user = '123' OR data_header.to_user = '456')",
		},
		{
			name:  "top-level field method",
			input: "method = 'INVITE'",
			want:  "method = 'INVITE'",
		},
		{
			name:  "top-level field status number",
			input: "status = 200",
			want:  "status = 200",
		},
		{
			name:  "not-equal operator",
			input: "status != 200",
			want:  "status != 200",
		},
		{
			name:  "call_id alias to sid",
			input: "call_id = 'abc123@host'",
			want:  "sid = 'abc123@host'",
		},
		{
			name:  "sid direct",
			input: "sid = 'abc123@host'",
			want:  "sid = 'abc123@host'",
		},
		{
			name:  "complex nested",
			input: "(from_user = '100' OR from_user = '200') AND method = 'INVITE'",
			want:  "(data_header.from_user = '100' OR data_header.from_user = '200') AND method = 'INVITE'",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "",
		},
		{
			name:    "unknown field",
			input:   "bogus = '123'",
			wantErr: "unknown field",
		},
		{
			name:    "missing value",
			input:   "from_user =",
			wantErr: "expected value",
		},
		{
			name:    "missing operator",
			input:   "from_user '123'",
			wantErr: "expected operator",
		},
		{
			name:    "unterminated string",
			input:   "from_user = '123",
			wantErr: "unterminated string",
		},
		{
			name:    "missing closing paren",
			input:   "(from_user = '123'",
			wantErr: "missing closing parenthesis",
		},
		{
			name:    "unexpected token after expression",
			input:   "from_user = '123' from_user = '456'",
			wantErr: "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQuery(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseQuery(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}
