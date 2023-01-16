package sha1

import "testing"

func TestHash_HexDigest(t *testing.T) {
	tt := []struct {
		username string
		hash     string
	}{
		{
			username: "Notch",
			hash:     "4ed1f46bbe04bc756bcb17c0c7ce3e4632f06a48",
		},
		{
			username: "jeb_",
			hash:     "-7c9d5b0044c130109a5d7b5fb5c317c02b4e28c1",
		},
		{
			username: "simon",
			hash:     "88e16a1019277b15d58faf0541e11910eb756f6",
		},
	}

	for _, test := range tt {
		hash := NewHash()
		hash.Update([]byte(test.username))
		if test.hash != hash.HexDigest() {
			t.Errorf("HexDigest of %s should be %s; got: %s", test.username, test.hash, hash)
		}
	}
}
