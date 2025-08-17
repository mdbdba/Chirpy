package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if hash == "" {
		t.Fatal("Expected non-empty hash")
	}

	if hash == password {
		t.Fatal("Hash should not be the same as the original password")
	}

	// bcrypt hashes should start with $2a$ or similar
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("Expected bcrypt hash format, got %s", hash)
	}
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("Expected no error for empty password, got %v", err)
	}

	if hash == "" {
		t.Fatal("Expected non-empty hash even for empty password")
	}
}

func TestCheckPasswordHash_ValidPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	err = CheckPasswordHash(password, hash)
	if err != nil {
		t.Errorf("Expected no error for valid password, got %v", err)
	}
}

func TestCheckPasswordHash_InvalidPassword(t *testing.T) {
	password := "testpassword123"
	wrongPassword := "wrongpassword456"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	err = CheckPasswordHash(wrongPassword, hash)
	if err == nil {
		t.Error("Expected error for invalid password, got none")
	}
}

func TestCheckPasswordHash_EmptyPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	err = CheckPasswordHash("", hash)
	if err == nil {
		t.Error("Expected error for empty password against non-empty hash, got none")
	}
}

func TestCheckPasswordHash_EmptyHash(t *testing.T) {
	password := "testpassword123"

	err := CheckPasswordHash(password, "")
	if err == nil {
		t.Error("Expected error for empty hash, got none")
	}
}

func TestCheckPasswordHash_InvalidHash(t *testing.T) {
	password := "testpassword123"
	invalidHash := "not-a-valid-bcrypt-hash"

	err := CheckPasswordHash(password, invalidHash)
	if err == nil {
		t.Error("Expected error for invalid hash format, got none")
	}
}

func TestHashPassword_Consistency(t *testing.T) {
	password := "testpassword123"

	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)

	if err1 != nil || err2 != nil {
		t.Fatalf("Expected no errors, got %v and %v", err1, err2)
	}

	// bcrypt should produce different hashes for the same password (due to salt)
	if hash1 == hash2 {
		t.Error("Expected different hashes for the same password (bcrypt uses random salt)")
	}

	// But both should validate correctly
	if err := CheckPasswordHash(password, hash1); err != nil {
		t.Errorf("First hash failed validation: %v", err)
	}

	if err := CheckPasswordHash(password, hash2); err != nil {
		t.Errorf("Second hash failed validation: %v", err)
	}
}

func TestPasswordHashing_RoundTrip(t *testing.T) {
	testPasswords := []string{
		"simple",
		"ComplexPassword123!@#",
		"password with spaces",
		"🔒🗝️emoji password",
		//"very_long_password_with_lots_of_characters_to_test_edge_cases_1234567890",
		"",
	}

	for _, password := range testPasswords {
		t.Run("password_"+password, func(t *testing.T) {
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("Failed to hash password '%s': %v", password, err)
			}

			err = CheckPasswordHash(password, hash)
			if err != nil {
				t.Errorf("Failed to validate password '%s': %v", password, err)
			}

			// Test with wrong password - use a different approach for empty passwords
			var wrongPassword string
			if password == "" {
				wrongPassword = "nonempty"
			} else {
				wrongPassword = password + "wrong"
			}

			err = CheckPasswordHash(wrongPassword, hash)
			if err == nil {
				t.Errorf("Expected error for wrong password '%s' against hash of '%s', but validation passed", wrongPassword, password)
			}
		})
	}
}
