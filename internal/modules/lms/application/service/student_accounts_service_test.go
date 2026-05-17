package service

import "testing"

func TestValidateStudentUsername(t *testing.T) {
	valid := []string{"student01", "student.01", "student_01", "student-01"}
	for _, username := range valid {
		if err := validateStudentUsername(username); err != nil {
			t.Fatalf("validateStudentUsername(%q) unexpected error: %v", username, err)
		}
	}

	invalid := []string{
		"st",
		"student@example.com",
		"student/01",
		".student",
		"student.",
		"student..01",
		"student 01",
	}
	for _, username := range invalid {
		if err := validateStudentUsername(normalizeStudentUsername(username)); err == nil {
			t.Fatalf("validateStudentUsername(%q) expected error", username)
		}
	}
}

func TestValidateStudentPassword(t *testing.T) {
	valid := []string{"ERG123456", "Classroom2026!"}
	for _, password := range valid {
		if err := validateStudentPassword(password, "student01", "Nguyen Van A"); err != nil {
			t.Fatalf("validateStudentPassword(%q) unexpected error: %v", password, err)
		}
	}

	invalid := []string{
		"123456",
		"abcdefgh",
		"student01A1",
		"nguyenvana1",
		"pass\nword1",
	}
	for _, password := range invalid {
		if err := validateStudentPassword(password, "student01", "Nguyen Van A"); err == nil {
			t.Fatalf("validateStudentPassword(%q) expected error", password)
		}
	}
}

func TestSecureTempPasswordMeetsStudentPolicy(t *testing.T) {
	password, err := secureTempPassword()
	if err != nil {
		t.Fatalf("secureTempPassword: %v", err)
	}
	if password == "123456" {
		t.Fatal("secureTempPassword returned legacy weak default")
	}
	if err := validateStudentPassword(password, "student01", "Nguyen Van A"); err != nil {
		t.Fatalf("secureTempPassword does not meet student policy: %v", err)
	}
}
