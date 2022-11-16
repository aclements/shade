package main

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
)

type CacheKey struct {
	key string
}

func MakeCacheKey(args ...any) *CacheKey {
	h := sha256.New()

	enc := gob.NewEncoder(h)
	for _, arg := range args {
		if err := enc.Encode(arg); err != nil {
			panic("error encoding cache key: " + err.Error())
		}
	}

	return &CacheKey{hex.EncodeToString(h.Sum(nil))}
}

func (ck *CacheKey) path() string {
	return filepath.Join(".cache", ck.key)
}

func (ck *CacheKey) Load(out any) bool {
	f, err := os.Open(ck.path())
	if err != nil {
		return false
	}
	defer f.Close()
	dec := gob.NewDecoder(f)
	if dec.Decode(out) != nil {
		return false
	}
	return true
}

func (ck *CacheKey) Save(val any) {
	if err := os.MkdirAll(".cache", 0777); err != nil {
		log.Printf("error creating .cache: %s", err)
		return
	}
	f, err := os.Create(ck.path())
	if err != nil {
		log.Printf("error saving to cache: %s", err)
		return
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	if err := enc.Encode(val); err != nil {
		panic("error encoding cache value: " + err.Error())
	}
}
