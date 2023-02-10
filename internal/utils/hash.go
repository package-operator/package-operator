package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"

	"github.com/davecgh/go-spew/spew"
	"k8s.io/apimachinery/pkg/util/rand"
)

// ComputeHash returns a fnv32 hash value calculated from pod template and
// a collisionCount to avoid hash collision. The hash will be safe encoded to
// avoid bad words.
func ComputeFNV32Hash(obj interface{}, collisionCount *int32) string {
	hasher := fnv.New32a()
	DeepHashObject(hasher, obj)

	// Add collisionCount in the hash if it exists.
	if collisionCount != nil {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(
			collisionCountBytes, uint32(*collisionCount))
		hasher.Write(collisionCountBytes)
	}

	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// ComputeHash returns a sha236 hash value calculated from pod template and
// a collisionCount to avoid hash collision. The hash will be safe encoded to
// avoid bad words.
func ComputeSHA256Hash(obj interface{}, collisionCount *int32) string {
	hasher := sha256.New()
	DeepHashObject(hasher, obj)

	// Add collisionCount in the hash if it exists.
	if collisionCount != nil {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(
			collisionCountBytes, uint32(*collisionCount))
		hasher.Write(collisionCountBytes)
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// DeepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", objectToWrite)
}
