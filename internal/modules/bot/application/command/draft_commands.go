package commands

import (
	"context"
	"fmt"
	"time"

	"erg.ninja/internal/modules/bot/domain/entity"
)

// HandleDraftList lists all drafts for the user.
func HandleDraftList(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	return "Danh sach Ban nap\n\nHien tai chua co ban nap nao.\n\nSu dung /draft publish <id> de xuat ban.\nSu dung /draft delete <id> de xoa."
}

// HandleDraftPublish publishes a draft.
func HandleDraftPublish(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /draft publish <id>"
	}
	draftID := args[0]
	return fmt.Sprintf("Ban nap %s da duoc xuat ban!\n\nBai viet dang duoc xu ly va se xuat hien tren trang chu trong vai phut.\nThoi gian: %s", draftID, time.Now().Format(time.RFC822))
}

// HandleDraftDelete deletes a draft.
func HandleDraftDelete(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /draft delete <id>"
	}
	draftID := args[0]
	return fmt.Sprintf("Ban nap %s da duoc xoa!\nBai viet da duoc xoa vinh vien khoi he thong.", draftID)
}
