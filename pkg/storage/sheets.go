package storage

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type GoogleSheetsClient struct {
	service *sheets.Service
}

func NewGoogleSheetsClient(ctx context.Context, credentialJSON string) (*GoogleSheetsClient, error) {
	var opts []option.ClientOption
	if credentialJSON != "" {
		if strings.HasPrefix(strings.TrimSpace(credentialJSON), "{") {
			opts = append(opts, option.WithCredentialsJSON([]byte(credentialJSON)))
		} else {
			opts = append(opts, option.WithCredentialsFile(credentialJSON))
		}
	}
	srv, err := sheets.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("sheets.new_service: %w", err)
	}
	return &GoogleSheetsClient{service: srv}, nil
}

func (c *GoogleSheetsClient) Tabs(ctx context.Context, spreadsheetID string) ([]SheetTab, error) {
	ss, err := c.service.Spreadsheets.Get(spreadsheetID).Fields("sheets(properties(sheetId,title,index))").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("sheets.tabs: %w", err)
	}
	tabs := make([]SheetTab, 0, len(ss.Sheets))
	for _, sheet := range ss.Sheets {
		if sheet.Properties == nil {
			continue
		}
		tabs = append(tabs, SheetTab{
			SheetID: sheet.Properties.SheetId,
			Title:   sheet.Properties.Title,
			Index:   sheet.Properties.Index,
		})
	}
	return tabs, nil
}

func (c *GoogleSheetsClient) Values(ctx context.Context, spreadsheetID, readRange string) ([][]string, error) {
	resp, err := c.service.Spreadsheets.Values.Get(spreadsheetID, readRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("sheets.values: %w", err)
	}
	rows := make([][]string, 0, len(resp.Values))
	for _, raw := range resp.Values {
		row := make([]string, len(raw))
		for i, cell := range raw {
			row[i] = fmt.Sprint(cell)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (c *GoogleSheetsClient) UpdateValues(ctx context.Context, spreadsheetID, writeRange string, values [][]any) error {
	_, err := c.service.Spreadsheets.Values.Update(spreadsheetID, writeRange, &sheets.ValueRange{
		Values: values,
	}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("sheets.update_values: %w", err)
	}
	return nil
}

type SheetTab struct {
	SheetID int64
	Title   string
	Index   int64
}
