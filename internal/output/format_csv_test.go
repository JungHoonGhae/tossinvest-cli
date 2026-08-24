package output

import (
	"errors"
	"strings"
	"testing"
)

type failingWriter struct {
	after int
	n     int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.n++
	if f.n > f.after {
		return 0, errors.New("boom")
	}
	return len(p), nil
}

func TestWriteCSVEmitsHeaderThenRows(t *testing.T) {
	var buf strings.Builder
	err := writeCSV(&buf, []string{"a", "b"}, [][]string{{"1", "2"}, {"3", "4"}})
	if err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	want := "a,b\n1,2\n3,4\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

// csv.Writer 는 버퍼링한다 — Flush() 는 실패를 돌려주지 않고 Error() 가 돌려준다.
// 이걸 빠뜨리면 출력이 조용히 잘린 채 성공으로 보고된다. 손으로 쓰던 40여 곳이
// 각자 기억해야 했던 규칙이라 여기서 고정한다.
func TestWriteCSVSurfacesFlushFailure(t *testing.T) {
	// 헤더는 통과시키고 그 뒤 쓰기에서 실패시킨다.
	err := writeCSV(&failingWriter{after: 0}, []string{"a"}, [][]string{{"1"}})
	if err == nil {
		t.Fatal("writeCSV must report a write failure, got nil")
	}
}

func TestWriteCSVWithNoRowsStillWritesHeader(t *testing.T) {
	var buf strings.Builder
	if err := writeCSV(&buf, []string{"a", "b"}, nil); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	if buf.String() != "a,b\n" {
		t.Errorf("empty result must still carry the header, got %q", buf.String())
	}
}
