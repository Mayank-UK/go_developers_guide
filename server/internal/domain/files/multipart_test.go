package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

// SaveMultipart over a fake store: the first FILE part streams in,
// non-file fields are skipped without derailing the parser, and every
// mis-shapen body answers with a typed error instead of a guess.

// fakeStore records the one save it performed.
type fakeStore struct {
	name    string
	limit   int64
	body    []byte
	saveErr error
}

func (f *fakeStore) SaveStream(_ context.Context, name string, body io.Reader, limit int64) (Object, error) {
	if f.saveErr != nil {
		return Object{}, f.saveErr
	}
	collected, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return Object{}, err
	}
	f.name, f.limit, f.body = name, limit, collected
	return Object{Name: name, Size: int64(len(collected))}, nil
}

func (f *fakeStore) Open(context.Context, string) (Object, io.ReadSeekCloser, error) {
	return Object{}, nil, ErrNotFound
}

func (f *fakeStore) Stat(context.Context, string) (Object, error) {
	return Object{}, ErrNotFound
}

func multipartBody(t *testing.T, parts ...func(w *multipart.Writer)) (string, *bytes.Buffer) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for _, part := range parts {
		part(writer)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return writer.FormDataContentType(), body
}

func TestSaveMultipartStreamsTheFirstFilePart(t *testing.T) {
	contentType, body := multipartBody(t,
		func(w *multipart.Writer) {
			field, err := w.CreateFormField("note")
			if err != nil {
				t.Fatal(err)
			}
			field.Write([]byte("a text field the route does not care about"))
		},
		func(w *multipart.Writer) {
			file, err := w.CreateFormFile("file", "payload.bin")
			if err != nil {
				t.Fatal(err)
			}
			file.Write(bytes.Repeat([]byte{0xAB}, 4096))
		},
	)
	store := &fakeStore{}
	object, err := SaveMultipart(context.Background(), store, contentType, body, 1<<20)
	if err != nil {
		t.Fatalf("SaveMultipart: %v", err)
	}
	if store.name != "payload.bin" {
		t.Errorf("saved name = %q, want the file part's filename", store.name)
	}
	if object.Size != 4096 || len(store.body) != 4096 {
		t.Errorf("saved size = %d/%d, want the 4096 payload bytes", object.Size, len(store.body))
	}
	if store.body[0] != 0xAB || store.body[4095] != 0xAB {
		t.Error("payload bytes drifted")
	}
}

func TestSaveMultipartRefusesBodiesWithoutFileParts(t *testing.T) {
	contentType, body := multipartBody(t, func(w *multipart.Writer) {
		field, err := w.CreateFormField("note")
		if err != nil {
			t.Fatal(err)
		}
		field.Write([]byte("text only"))
	})
	if _, err := SaveMultipart(context.Background(), &fakeStore{}, contentType, body, 1<<20); !errors.Is(err, ErrNoFilePart) {
		t.Errorf("err = %v, want ErrNoFilePart", err)
	}
	emptyType, emptyBody := multipartBody(t, func(*multipart.Writer) {})
	if _, err := SaveMultipart(context.Background(), &fakeStore{}, emptyType, emptyBody, 1<<20); !errors.Is(err, ErrNoFilePart) {
		t.Errorf("empty body err = %v, want ErrNoFilePart", err)
	}
}

func TestSaveMultipartRefusesNonMultipartContentTypes(t *testing.T) {
	for _, contentType := range []string{
		"application/json",
		"multipart/form-data",               // no boundary
		"multipart/mixed; boundary=x",       // wrong subtype
		"completely-broken; ;;;; boundary=", // unparseable
	} {
		if _, err := SaveMultipart(context.Background(), &fakeStore{}, contentType, strings.NewReader("x"), 1<<20); err == nil {
			t.Errorf("content type %q must be refused", contentType)
		}
	}
}

func TestSaveMultipartPropagatesStoreErrorsUntouched(t *testing.T) {
	contentType, body := multipartBody(t, func(w *multipart.Writer) {
		file, err := w.CreateFormFile("file", "hostile..name")
		if err != nil {
			t.Fatal(err)
		}
		file.Write([]byte("bytes"))
	})
	store := &fakeStore{saveErr: ErrInvalidName}
	if _, err := SaveMultipart(context.Background(), store, contentType, body, 1<<20); !errors.Is(err, ErrInvalidName) {
		t.Errorf("err = %v, want the store's ErrInvalidName verbatim", err)
	}
}
