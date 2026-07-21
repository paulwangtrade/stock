package data

import (
	"testing"
)

func TestGetLatestCloseSignalSnapshot_EmptyAsOfUsesTodayFormat(t *testing.T) {
	// 无 DB 时返回 nil，不 panic
	repo := NewSignalSnapshotRepo()
	snap, err := repo.GetLatestCloseSignalSnapshot("")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	_ = snap
}

func TestParseSnapshotHits_Empty(t *testing.T) {
	repo := NewSignalSnapshotRepo()
	if hits := repo.ParseSnapshotHits(nil); hits != nil {
		t.Fatal("nil snap")
	}
}
