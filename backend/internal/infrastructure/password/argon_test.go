package password

import "testing"

func TestArgon(t *testing.T) {
	hasher := Argon{}
	first, err := hasher.Hash("long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.Hash("long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("password salts were reused")
	}
	if !hasher.Verify(first, "long-enough-password") {
		t.Fatal("correct password rejected")
	}
	if hasher.Verify(first, "wrong-password") || hasher.Verify("", "long-enough-password") || hasher.Verify("$argon2id$v=19$m=999999999,t=3,p=1$bad$bad", "long-enough-password") {
		t.Fatal("invalid credentials accepted")
	}
}
