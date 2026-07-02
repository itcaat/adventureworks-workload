package app

import (
	"database/sql"
	"testing"
)

func TestRequestBytesCountsQueryAndParameters(t *testing.T) {
	query := "SELECT TOP (@limit) Name FROM Production.Product WHERE Color = @color;"
	got := requestBytes(query,
		sql.Named("limit", 25),
		sql.Named("color", "Blue"),
	)
	if got <= int64(len(query)) {
		t.Fatalf("requestBytes() = %d, want > query length %d", got, len(query))
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{512, "512 B"},
		{2048, "2.0 KB"},
		{5 << 20, "5.0 MB"},
	}
	for _, tt := range tests {
		if got := FormatBytes(tt.n); got != tt.want {
			t.Fatalf("FormatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
