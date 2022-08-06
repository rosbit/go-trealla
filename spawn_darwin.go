package trealla

import (
	"github.com/rosbit/go-expect"
)

func spawn(treallaExePath string) (e *expect.Expect, err error) {
	return expect.SpawnPTY(treallaExePath)
}
