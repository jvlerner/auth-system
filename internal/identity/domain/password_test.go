package domain

import (
	"testing"
)

func TestRestorePassword_Success(t *testing.T) {
	hash := "$argon2id$v=19$m=65536,t=1,p=4$salt$hash"
	password, err := RestorePassword(hash)
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	if password.Hash() != hash {
		t.Errorf("expected Hash() to return %v, got %v", hash, password.Hash())
	}
	
	if password.Value() != hash {
		t.Errorf("expected Value() to return %v, got %v", hash, password.Value())
	}
}

func TestRestorePassword_EmptyStr(t *testing.T) {
	_, err := RestorePassword("")
	
	if err == nil {
		t.Error("expected error for empty hash, got nil")
	}
}
