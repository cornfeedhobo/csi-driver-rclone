package csirclone

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
	"storj.io/common/base58"
)

var MetadataFilename = ".csi-metadata"

type Volume struct {
	Remote   string `json:"remote"`
	Name     string `json:"name"`
	Capacity int64  `json:"capacity"`
	ID       string `json:"id"`
}

func NewVolume(remote, name string, capacity int64) *Volume {

	hasher := sha1.New()
	hasher.Write([]byte(name))

	sum := base58.Encode(hasher.Sum(nil))

	return &Volume{
		Remote:   remote,
		Name:     name,
		Capacity: capacity,
		ID:       sum,
	}
}

func NewVolumeFromJSON(metadata []byte) (*Volume, error) {

	v := &Volume{}

	err := json.Unmarshal(metadata, v)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling json: %w", err)
	}

	return v, nil
}

func (v *Volume) IsConflict(new *Volume) error {

	if v.ID != new.ID {
		return ErrMetaWrongID
	}

	if v.Capacity != new.Capacity {
		// This is required to pass the CSI test suite, even though rclone can't enforce capacity.
		return ErrMetaWrongCapacity
	}

	return nil
}

func (v *Volume) Marshal(indent bool) ([]byte, error) {
	switch indent {
	case true:
		return json.MarshalIndent(v, "", "\t")
	default:
		return json.Marshal(v)
	}
}

func (v *Volume) Unmarshal(b []byte) error {
	return json.Unmarshal(b, v)
}

// VolumeLocks implements a map with atomic operations.
// It stores a set of all volume IDs with an ongoing operation.
type VolumeLocks struct {
	locks sets.String
	mux   sync.Mutex
}

func NewVolumeLocks() *VolumeLocks {
	return &VolumeLocks{
		locks: sets.NewString(),
	}
}

// TryAcquire tries to acquire the lock for operating on volumeID and returns true if successful.
// If another operation is already using volumeID, returns false.
func (l *VolumeLocks) TryAcquire(volumeID string) bool {
	l.mux.Lock()
	defer l.mux.Unlock()
	if l.locks.Has(volumeID) {
		return false
	}
	l.locks.Insert(volumeID)
	return true
}

// Release the lock on the specified volumeID
func (l *VolumeLocks) Release(volumeID string) {
	l.mux.Lock()
	defer l.mux.Unlock()
	l.locks.Delete(volumeID)
}
