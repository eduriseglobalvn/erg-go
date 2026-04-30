package lms

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"erg.ninja/pkg/storage"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var spreadsheetIDPattern = regexp.MustCompile(`/spreadsheets/d/([a-zA-Z0-9-_]+)`)

func (s *Service) GoogleSheetTabs(ctx context.Context, req GoogleSheetTabsRequestDTO) (GoogleSheetTabsResponseDTO, error) {
	if s.sheets == nil {
		return GoogleSheetTabsResponseDTO{}, errSheetAccess
	}
	spreadsheetID, err := spreadsheetIDFromURL(req.SheetURL)
	if err != nil {
		return GoogleSheetTabsResponseDTO{}, err
	}
	tabs, err := s.sheets.Tabs(ctx, spreadsheetID)
	if err != nil {
		return GoogleSheetTabsResponseDTO{}, errSheetAccess
	}
	return GoogleSheetTabsResponseDTO{SpreadsheetID: spreadsheetID, Tabs: sheetTabsToDTO(tabs)}, nil
}

func (s *Service) PreviewGoogleSheet(ctx context.Context, tenantID string, actor Actor, req GoogleSheetPreviewRequestDTO) (GoogleSheetPreviewResponseDTO, error) {
	if s.sheets == nil {
		return GoogleSheetPreviewResponseDTO{}, errSheetAccess
	}
	spreadsheetID, err := spreadsheetIDFromURL(req.SheetURL)
	if err != nil {
		return GoogleSheetPreviewResponseDTO{}, err
	}
	values, err := s.sheets.Values(ctx, spreadsheetID, sheetRange(req.SheetName, req.Range))
	if err != nil {
		return GoogleSheetPreviewResponseDTO{}, errSheetAccess
	}
	rows, warnings := parseStudentRows(values, req.Mapping)
	preview := &ImportPreview{
		TenantID:      tenantID,
		UserID:        actor.UserID,
		SpreadsheetID: spreadsheetID,
		SheetName:     req.SheetName,
		Range:         req.Range,
		Rows:          rows,
	}
	if err := s.repo.CreateImportPreview(ctx, preview); err != nil {
		return GoogleSheetPreviewResponseDTO{}, err
	}
	return previewToResponse(*preview, warnings), nil
}

func (s *Service) CommitGoogleSheetImport(ctx context.Context, tenantID string, actor Actor, req GoogleSheetCommitRequestDTO) (GoogleSheetCommitResponseDTO, error) {
	preview, err := s.repo.GetImportPreview(ctx, tenantID, req.PreviewID)
	if err != nil {
		return GoogleSheetCommitResponseDTO{}, err
	}
	if preview == nil {
		return GoogleSheetCommitResponseDTO{}, errNotFound
	}
	center, err := s.repo.GetCenter(ctx, tenantID, req.CenterID)
	if err != nil {
		return GoogleSheetCommitResponseDTO{}, err
	}
	if center == nil {
		return GoogleSheetCommitResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, req.CenterID) {
		return GoogleSheetCommitResponseDTO{}, errScopeForbidden
	}
	rowByID := map[string]ParsedStudentRow{}
	for _, row := range preview.Rows {
		rowByID[row.RowID] = row
	}
	job := &ImportJob{
		TenantID:      tenantID,
		UserID:        actor.UserID,
		PreviewID:     preview.ID,
		SpreadsheetID: preview.SpreadsheetID,
		SheetName:     preview.SheetName,
		SheetRange:    preview.Range,
		Status:        "completed",
		Progress:      100,
		Errors:        []string{},
		Credentials:   []ImportCredential{},
	}
	for _, commitRow := range req.Rows {
		if !commitRow.Included {
			job.Skipped++
			continue
		}
		parsed := rowByID[commitRow.RowID]
		merged := mergeCommitRow(parsed, commitRow)
		if merged.FullName == "" {
			job.Skipped++
			job.Errors = append(job.Errors, fmt.Sprintf("%s: missing fullName", commitRow.RowID))
			continue
		}
		class, err := s.resolveImportClass(ctx, tenantID, center.ID, req.ClassID, merged.ClassName)
		if err != nil {
			job.Skipped++
			job.Errors = append(job.Errors, fmt.Sprintf("%s: %s", commitRow.RowID, err.Error()))
			continue
		}
		username, duplicate, err := s.uniqueStudentUsername(ctx, tenantID, merged.FullName, class.Name)
		if err != nil {
			return GoogleSheetCommitResponseDTO{}, err
		}
		if duplicate {
			job.Duplicates++
		}
		password := tempImportPassword()
		student := &Student{
			TenantID:  tenantID,
			CenterID:  class.CenterID,
			ClassID:   class.ID,
			FullName:  merged.FullName,
			Username:  username,
			Birthday:  merged.Birthday,
			Phone:     merged.Phone,
			Note:      merged.Note,
			Status:    statusActive,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := s.repo.CreateStudent(ctx, student); err != nil {
			job.Skipped++
			job.Errors = append(job.Errors, fmt.Sprintf("%s: %s", commitRow.RowID, err.Error()))
			continue
		}
		job.Created++
		job.Credentials = append(job.Credentials, ImportCredential{
			RowID:     commitRow.RowID,
			RowNumber: merged.RowNumber,
			StudentID: student.ID.Hex(),
			Username:  username,
			Password:  password,
		})
	}
	if err := s.repo.CreateImportJob(ctx, job); err != nil {
		return GoogleSheetCommitResponseDTO{}, err
	}
	return GoogleSheetCommitResponseDTO{
		JobID:       job.ID.Hex(),
		Created:     job.Created,
		Skipped:     job.Skipped,
		Duplicates:  job.Duplicates,
		Credentials: credentialsToDTO(job.Credentials),
	}, nil
}

func (s *Service) GetImportJob(ctx context.Context, tenantID, jobID string) (ImportJobResponseDTO, error) {
	job, err := s.repo.GetImportJob(ctx, tenantID, jobID)
	if err != nil {
		return ImportJobResponseDTO{}, err
	}
	if job == nil {
		return ImportJobResponseDTO{}, errNotFound
	}
	return ImportJobResponseDTO{
		JobID:     job.ID.Hex(),
		Status:    job.Status,
		Progress:  job.Progress,
		Created:   job.Created,
		Skipped:   job.Skipped,
		Errors:    job.Errors,
		UpdatedAt: job.UpdatedAt,
	}, nil
}

func (s *Service) WritebackImportJob(ctx context.Context, tenantID, jobID string, req SheetWritebackRequestDTO) (SheetWritebackResponseDTO, error) {
	job, err := s.repo.GetImportJob(ctx, tenantID, jobID)
	if err != nil {
		return SheetWritebackResponseDTO{}, err
	}
	if job == nil {
		return SheetWritebackResponseDTO{}, errNotFound
	}
	if req.WriteMode == "download-csv" {
		return SheetWritebackResponseDTO{UpdatedRows: len(job.Credentials), DownloadURL: csvDataURL(job.Credentials)}, nil
	}
	if s.sheets == nil {
		return SheetWritebackResponseDTO{}, errSheetAccess
	}
	for _, cred := range job.Credentials {
		values := [][]any{{cred.Username, cred.Password}}
		writeRange := fmt.Sprintf("%s!%s%d:%s%d", quoteSheetName(job.SheetName), strings.ToUpper(req.UsernameColumn), cred.RowNumber, strings.ToUpper(req.PasswordColumn), cred.RowNumber)
		if err := s.sheets.UpdateValues(ctx, job.SpreadsheetID, writeRange, values); err != nil {
			return SheetWritebackResponseDTO{}, errSheetAccess
		}
	}
	now := time.Now().UTC()
	return SheetWritebackResponseDTO{UpdatedRows: len(job.Credentials), SheetUpdatedAt: &now}, nil
}

func (s *Service) resolveImportClass(ctx context.Context, tenantID string, centerID bson.ObjectID, classID, className string) (*Class, error) {
	if classID != "" {
		class, err := s.repo.GetClass(ctx, tenantID, classID)
		if err != nil {
			return nil, err
		}
		if class == nil || class.CenterID != centerID {
			return nil, errNotFound
		}
		return class, nil
	}
	if className == "" {
		return nil, fmt.Errorf("missing className")
	}
	class, err := s.repo.GetClassByName(ctx, tenantID, centerID, className)
	if err != nil {
		return nil, err
	}
	if class == nil {
		return nil, fmt.Errorf("class %q not found", className)
	}
	return class, nil
}

func (s *Service) uniqueStudentUsername(ctx context.Context, tenantID, fullName, className string) (string, bool, error) {
	base := normalizeUsername(fmt.Sprintf("%s.%s", fullName, className))
	if base == "" {
		base = "student"
	}
	username := base
	duplicate := false
	for i := 0; i < 100; i++ {
		exists, err := s.repo.UsernameExists(ctx, tenantID, username)
		if err != nil {
			return "", false, err
		}
		if !exists {
			return username, duplicate, nil
		}
		duplicate = true
		username = fmt.Sprintf("%s.%d", base, i+1)
	}
	return "", duplicate, fmt.Errorf("could not generate unique username")
}

func spreadsheetIDFromURL(sheetURL string) (string, error) {
	matches := spreadsheetIDPattern.FindStringSubmatch(sheetURL)
	if len(matches) == 2 {
		return matches[1], nil
	}
	u, err := url.Parse(sheetURL)
	if err == nil {
		if id := u.Query().Get("id"); id != "" {
			return id, nil
		}
	}
	if strings.TrimSpace(sheetURL) != "" && !strings.Contains(sheetURL, "/") {
		return strings.TrimSpace(sheetURL), nil
	}
	return "", fmt.Errorf("invalid sheetUrl")
}

func sheetRange(sheetName, readRange string) string {
	if strings.Contains(readRange, "!") {
		return readRange
	}
	return fmt.Sprintf("%s!%s", quoteSheetName(sheetName), readRange)
}

func quoteSheetName(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func parseStudentRows(values [][]string, mapping GoogleSheetPreviewMappingDTO) ([]ParsedStudentRow, []string) {
	if len(values) == 0 {
		return nil, []string{"sheet range has no rows"}
	}
	headers := values[0]
	index := buildHeaderIndex(headers)
	fullNameCol := mappedColumn(mapping.FullName, index, "full name", "fullname", "ho ten", "họ tên", "name")
	familyCol := mappedColumn(mapping.FamilyName, index, "ho", "họ", "family name")
	givenCol := mappedColumn(mapping.GivenName, index, "ten", "tên", "given name")
	classCol := mappedColumn(mapping.ClassName, index, "class", "class name", "lop", "lớp")
	birthdayCol := mappedColumn(mapping.Birthday, index, "birthday", "dob", "ngay sinh", "ngày sinh")
	phoneCol := mappedColumn(mapping.Phone, index, "phone", "sdt", "số điện thoại")
	noteCol := mappedColumn(mapping.Note, index, "note", "ghi chu", "ghi chú")
	warnings := []string{}
	if fullNameCol < 0 && (familyCol < 0 || givenCol < 0) {
		warnings = append(warnings, "fullName mapping not detected")
	}
	if classCol < 0 {
		warnings = append(warnings, "className mapping not detected")
	}
	rows := []ParsedStudentRow{}
	for i := 1; i < len(values); i++ {
		raw := values[i]
		fullName := cell(raw, fullNameCol)
		if fullName == "" {
			fullName = strings.TrimSpace(cell(raw, familyCol) + " " + cell(raw, givenCol))
		}
		row := ParsedStudentRow{
			RowID:     fmt.Sprintf("row-%d", i+1),
			RowNumber: i + 1,
			FullName:  fullName,
			ClassName: cell(raw, classCol),
			Birthday:  parseDate(cell(raw, birthdayCol)),
			Phone:     cell(raw, phoneCol),
			Note:      cell(raw, noteCol),
			Status:    "valid",
			Messages:  []string{},
		}
		if row.FullName == "" {
			row.Status = "error"
			row.Messages = append(row.Messages, "missing fullName")
		}
		if row.ClassName == "" {
			if row.Status != "error" {
				row.Status = "warning"
			}
			row.Messages = append(row.Messages, "missing className")
		}
		rows = append(rows, row)
	}
	return rows, warnings
}

func buildHeaderIndex(headers []string) map[string]int {
	index := map[string]int{}
	for i, header := range headers {
		index[normalizeHeader(header)] = i
	}
	return index
}

func mappedColumn(mapped string, index map[string]int, aliases ...string) int {
	if mapped != "" {
		if n, ok := index[normalizeHeader(mapped)]; ok {
			return n
		}
	}
	for _, alias := range aliases {
		if n, ok := index[normalizeHeader(alias)]; ok {
			return n
		}
	}
	return -1
}

func cell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseDate(value string) *time.Time {
	if value == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2/1/2006", "01/02/2006"} {
		if t, err := time.Parse(layout, value); err == nil {
			return &t
		}
	}
	return nil
}

func previewToResponse(preview ImportPreview, warnings []string) GoogleSheetPreviewResponseDTO {
	rows := make([]ParsedStudentRowDTO, 0, len(preview.Rows))
	classes := map[string]bool{}
	summary := GoogleSheetPreviewSummaryDTO{TotalRows: len(preview.Rows)}
	errors := []string{}
	for _, row := range preview.Rows {
		rows = append(rows, parsedRowToDTO(row))
		if row.ClassName != "" {
			classes[row.ClassName] = true
		}
		switch row.Status {
		case "error":
			summary.ErrorRows++
			errors = append(errors, fmt.Sprintf("%s: %s", row.RowID, strings.Join(row.Messages, ", ")))
		case "warning":
			summary.WarningRows++
			summary.IncludedRows++
		default:
			summary.ValidRows++
			summary.IncludedRows++
		}
	}
	detected := make([]string, 0, len(classes))
	for className := range classes {
		detected = append(detected, className)
	}
	sort.Strings(detected)
	return GoogleSheetPreviewResponseDTO{PreviewID: preview.ID.Hex(), Rows: rows, DetectedClasses: detected, Errors: errors, Warnings: warnings, Summary: summary}
}

func parsedRowToDTO(row ParsedStudentRow) ParsedStudentRowDTO {
	return ParsedStudentRowDTO{RowID: row.RowID, RowNumber: row.RowNumber, FullName: row.FullName, ClassName: row.ClassName, Birthday: row.Birthday, Phone: row.Phone, Note: row.Note, Status: row.Status, Messages: row.Messages}
}

func mergeCommitRow(parsed ParsedStudentRow, commit GoogleSheetCommitRowDTO) ParsedStudentRow {
	if commit.FullName != "" {
		parsed.FullName = commit.FullName
	}
	if commit.ClassName != "" {
		parsed.ClassName = commit.ClassName
	}
	if commit.Birthday != nil {
		parsed.Birthday = commit.Birthday
	}
	if commit.Phone != "" {
		parsed.Phone = commit.Phone
	}
	if commit.Note != "" {
		parsed.Note = commit.Note
	}
	return parsed
}

func sheetTabsToDTO(tabs []storage.SheetTab) []GoogleSheetTabDTO {
	out := make([]GoogleSheetTabDTO, 0, len(tabs))
	for _, tab := range tabs {
		out = append(out, GoogleSheetTabDTO{SheetID: tab.SheetID, Title: tab.Title, Index: tab.Index})
	}
	return out
}

func credentialsToDTO(items []ImportCredential) []ImportCredentialDTO {
	out := make([]ImportCredentialDTO, 0, len(items))
	for _, item := range items {
		out = append(out, ImportCredentialDTO{RowID: item.RowID, RowNumber: item.RowNumber, StudentID: item.StudentID, Username: item.Username, Password: item.Password})
	}
	return out
}

func normalizeHeader(value string) string {
	return normalizeUsername(value)
}

func normalizeUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDot := false
	for _, r := range value {
		r = foldVietnamese(r)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDot = false
			continue
		}
		if !lastDot {
			b.WriteByte('.')
			lastDot = true
		}
	}
	return strings.Trim(b.String(), ".")
}

func foldVietnamese(r rune) rune {
	switch r {
	case 'à', 'á', 'ả', 'ã', 'ạ', 'ă', 'ằ', 'ắ', 'ẳ', 'ẵ', 'ặ', 'â', 'ầ', 'ấ', 'ẩ', 'ẫ', 'ậ':
		return 'a'
	case 'è', 'é', 'ẻ', 'ẽ', 'ẹ', 'ê', 'ề', 'ế', 'ể', 'ễ', 'ệ':
		return 'e'
	case 'ì', 'í', 'ỉ', 'ĩ', 'ị':
		return 'i'
	case 'ò', 'ó', 'ỏ', 'õ', 'ọ', 'ô', 'ồ', 'ố', 'ổ', 'ỗ', 'ộ', 'ơ', 'ờ', 'ớ', 'ở', 'ỡ', 'ợ':
		return 'o'
	case 'ù', 'ú', 'ủ', 'ũ', 'ụ', 'ư', 'ừ', 'ứ', 'ử', 'ữ', 'ự':
		return 'u'
	case 'ỳ', 'ý', 'ỷ', 'ỹ', 'ỵ':
		return 'y'
	case 'đ':
		return 'd'
	default:
		return r
	}
}

func tempImportPassword() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ERG%d", time.Now().UnixNano()%1000000)
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return "ERG" + string(buf)
}

func csvDataURL(credentials []ImportCredential) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"rowId", "rowNumber", "studentId", "username", "password"})
	for _, cred := range credentials {
		_ = w.Write([]string{cred.RowID, fmt.Sprint(cred.RowNumber), cred.StudentID, cred.Username, cred.Password})
	}
	w.Flush()
	return "data:text/csv;charset=utf-8," + url.QueryEscape(b.String())
}
