package alarm

import (
	"strings"
	"testing"
)

func TestAzanDataNotEmpty(t *testing.T) {
	if len(azanData) == 0 {
		t.Fatal("azanData embed is empty")
	}
}

func TestAzanDataIsWebM(t *testing.T) {
	if len(azanData) < 4 {
		t.Fatal("azanData too short")
	}
	if !strings.HasPrefix(string(azanData[:4]), "\x1a\x45\xdf\xa3") {
		t.Fatal("azanData does not appear to be a WebM file (missing EBML header)")
	}
}
