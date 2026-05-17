package service

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxStudentCodeLength   = 64
	maxStudentNameLength   = 160
	maxStudentEmailLength  = 254
	maxStudentPhoneLength  = 32
	maxStudentAddressChars = 300
	maxStudentNoteChars    = 1000
)

var (
	studentCodePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$`)
	studentEmailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	studentPhonePattern = regexp.MustCompile(`^[+0-9][0-9 .()/-]{5,31}$`)
)

type normalizedStudentProfile struct {
	StudentCode        string
	FullName           string
	Email              string
	Gender             string
	Phone              string
	Address            string
	ParentName         string
	ParentPhone        string
	ParentEmail        string
	ParentRelationship string
	Note               string
}

func normalizeCreateStudentProfile(req CreateStudentRequestDTO) (normalizedStudentProfile, error) {
	return normalizeStudentProfile(normalizedStudentProfile{
		StudentCode:        req.StudentCode,
		FullName:           req.FullName,
		Email:              req.Email,
		Gender:             req.Gender,
		Phone:              req.Phone,
		Address:            req.Address,
		ParentName:         req.ParentName,
		ParentPhone:        req.ParentPhone,
		ParentEmail:        req.ParentEmail,
		ParentRelationship: req.ParentRelationship,
		Note:               req.Note,
	})
}

func normalizeBulkStudentProfile(row BulkCreateStudentAccountRowDTO) (normalizedStudentProfile, error) {
	return normalizeStudentProfile(normalizedStudentProfile{
		StudentCode:        row.StudentCode,
		FullName:           row.FullName,
		Email:              row.Email,
		Gender:             row.Gender,
		Phone:              row.Phone,
		Address:            row.Address,
		ParentName:         row.ParentName,
		ParentPhone:        row.ParentPhone,
		ParentEmail:        row.ParentEmail,
		ParentRelationship: row.ParentRelationship,
		Note:               row.Note,
	})
}

func normalizeParsedStudentProfile(row ParsedStudentRow) (normalizedStudentProfile, error) {
	return normalizeStudentProfile(normalizedStudentProfile{
		StudentCode:        row.StudentCode,
		FullName:           row.FullName,
		Email:              row.Email,
		Gender:             row.Gender,
		Phone:              row.Phone,
		Address:            row.Address,
		ParentName:         row.ParentName,
		ParentPhone:        row.ParentPhone,
		ParentEmail:        row.ParentEmail,
		ParentRelationship: row.ParentRelationship,
		Note:               row.Note,
	})
}

func normalizeStudentProfile(profile normalizedStudentProfile) (normalizedStudentProfile, error) {
	profile.StudentCode = strings.TrimSpace(profile.StudentCode)
	profile.FullName = strings.Join(strings.Fields(profile.FullName), " ")
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	profile.Gender = strings.ToLower(strings.TrimSpace(profile.Gender))
	profile.Phone = strings.TrimSpace(profile.Phone)
	profile.Address = strings.TrimSpace(profile.Address)
	profile.ParentName = strings.Join(strings.Fields(profile.ParentName), " ")
	profile.ParentPhone = strings.TrimSpace(profile.ParentPhone)
	profile.ParentEmail = strings.ToLower(strings.TrimSpace(profile.ParentEmail))
	profile.ParentRelationship = strings.ToLower(strings.TrimSpace(profile.ParentRelationship))
	profile.Note = strings.TrimSpace(profile.Note)

	if profile.FullName == "" || exceedsRunes(profile.FullName, maxStudentNameLength) || hasControlChars(profile.FullName) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if profile.StudentCode != "" && (!studentCodePattern.MatchString(profile.StudentCode) || exceedsRunes(profile.StudentCode, maxStudentCodeLength)) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if profile.Email != "" && (!studentEmailPattern.MatchString(profile.Email) || exceedsRunes(profile.Email, maxStudentEmailLength) || hasControlChars(profile.Email)) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if profile.ParentEmail != "" && (!studentEmailPattern.MatchString(profile.ParentEmail) || exceedsRunes(profile.ParentEmail, maxStudentEmailLength) || hasControlChars(profile.ParentEmail)) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if profile.Gender != "" && profile.Gender != "male" && profile.Gender != "female" && profile.Gender != "other" {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if profile.Phone != "" && (!studentPhonePattern.MatchString(profile.Phone) || exceedsRunes(profile.Phone, maxStudentPhoneLength) || hasControlChars(profile.Phone)) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if profile.ParentPhone != "" && (!studentPhonePattern.MatchString(profile.ParentPhone) || exceedsRunes(profile.ParentPhone, maxStudentPhoneLength) || hasControlChars(profile.ParentPhone)) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if exceedsRunes(profile.Address, maxStudentAddressChars) || exceedsRunes(profile.Note, maxStudentNoteChars) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	if hasControlChars(profile.Address) || hasControlChars(profile.ParentName) || hasControlChars(profile.ParentRelationship) || hasControlChars(profile.Note) {
		return normalizedStudentProfile{}, errInvalidStudentProfile
	}
	return profile, nil
}

func hasControlChars(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

func exceedsRunes(value string, limit int) bool {
	return utf8.RuneCountInString(value) > limit
}
