package entropy_id_panic_test

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy source unavailable")
}

// TestCreateCaseHandlesEntropyFailure verifies that an operating-system entropy
// failure is returned as an application error instead of terminating the
// process through a panic in case identifier generation.
func TestCreateCaseHandlesEntropyFailure(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "entropy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := workflow.NewService(repo)

	previous := cryptorand.Reader
	cryptorand.Reader = failingReader{}
	defer func() { cryptorand.Reader = previous }()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("CreateCase panic on entropy failure: %v", recovered)
		}
	}()

	_, _, err = service.CreateCase(context.Background(), workflow.CreateCaseCommand{
		RequestID:     "entropy-request-01",
		Title:         "熵源故障样本",
		Edition:       "1.0",
		MediaFormat:   "EPUB 3",
		OwnerID:       "owner",
		ContentDigest: strings.Repeat("a", 64),
	})
	if err == nil {
		t.Fatal("熵源失败应返回可处理错误")
	}
}
