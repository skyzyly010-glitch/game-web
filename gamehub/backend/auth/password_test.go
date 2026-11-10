package auth

import (
	"strings"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse battery" {
		t.Fatal("hash must not equal the plain password")
	}
	if !CheckPassword(hash, "correct horse battery") {
		t.Error("CheckPassword rejected the right password")
	}
	if CheckPassword(hash, "wrong password") {
		t.Error("CheckPassword accepted the wrong password")
	}
}

func TestHashIsSalted(t *testing.T) {
	a, _ := HashPassword("same password")
	b, _ := HashPassword("same password")
	if a == b {
		t.Fatal("two hashes of the same password are identical (no salt?)")
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"too short", "1234567", ErrPasswordTooShort},
		{"min length", "12345678", nil},
		{"max length", strings.Repeat("a", 72), nil},
		{"too long", strings.Repeat("a", 73), ErrPasswordTooLong},
		{"thai max counted in bytes", strings.Repeat("ก", 25), ErrPasswordTooLong}, // 25 x 3 ไบต์ = 75
		{"thai min counted in characters", "กขค", ErrPasswordTooShort},             // 3 ตัว = 9 ไบต์ แต่ยังสั้นเกิน
		{"thai 8 characters ok", "กขคงจฉชซ", nil},                                  // 8 ตัว = 24 ไบต์
	}
	for _, tt := range tests {
		if got := ValidatePassword(tt.in); got != tt.want {
			t.Errorf("%s: ValidatePassword = %v, want %v", tt.name, got, tt.want)
		}
	}
}
