package academy

import (
	"FinTalent/internal/objectstorage"
	"testing"
)

func TestMultipartPartsMustMatchServerReservation(t *testing.T) {
	s := uploadSession{Size: uploadPartSize + 10, PartSize: uploadPartSize}
	good := []objectstorage.Part{{Number: 1, Size: uploadPartSize, ETag: "a"}, {Number: 2, Size: 10, ETag: "b"}}
	if !validateParts(s, good) {
		t.Fatal("valid parts rejected")
	}
	for _, parts := range [][]objectstorage.Part{good[:1], {good[1], good[0]}, {{Number: 1, Size: uploadPartSize + 1, ETag: "a"}, good[1]}, {{Number: 1, Size: uploadPartSize, ETag: ""}, good[1]}} {
		if validateParts(s, parts) {
			t.Fatal("invalid parts accepted")
		}
	}
}
func TestUploadedVideoReadiness(t *testing.T) {
	p := learningFixture()
	l := &p.Modules[0].Lessons[0]
	l.Blocks = []Block{{Key: Key(), Type: "video", Data: BlockData{AssetID: 123}}}
	if len(CheckReadiness(p).Blockers) > 0 {
		t.Fatal(CheckReadiness(p))
	}
	l.Blocks[0].Data.AssetID = 0
	if len(CheckReadiness(p).Blockers) == 0 {
		t.Fatal("empty video passed readiness")
	}
}
