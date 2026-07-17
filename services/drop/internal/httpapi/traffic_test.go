package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

type readerFromRecorder struct {
	header         http.Header
	body           bytes.Buffer
	readFromCalled bool
}

func (w *readerFromRecorder) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *readerFromRecorder) WriteHeader(int) {}

func (w *readerFromRecorder) Write(content []byte) (int, error) {
	return w.body.Write(content)
}

func (w *readerFromRecorder) ReadFrom(src io.Reader) (int64, error) {
	w.readFromCalled = true
	return w.body.ReadFrom(src)
}

func TestTrafficResponseWriterPreservesReaderFrom(t *testing.T) {
	underlying := &readerFromRecorder{}
	meter := &trafficResponseWriter{ResponseWriter: underlying}
	content := []byte("attachment payload")

	written, err := io.CopyN(meter, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("copy response: %v", err)
	}
	if !underlying.readFromCalled {
		t.Fatal("underlying ReaderFrom was not used")
	}
	if written != int64(len(content)) || meter.bytes != int64(len(content)) {
		t.Fatalf("written=%d metered=%d, want %d", written, meter.bytes, len(content))
	}
	if meter.status != http.StatusOK {
		t.Fatalf("status=%d, want %d", meter.status, http.StatusOK)
	}
	if !bytes.Equal(underlying.body.Bytes(), content) {
		t.Fatal("copied content differs")
	}
}
