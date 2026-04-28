// Package menus provides hardcoded per-domain navigation menu trees.
package menus

import (
	"strings"

	"erg.ninja/pkg/logger"
)

// ─── Domain menu definitions ────────────────────────────────────────────────────

type menuItem struct {
	label    string
	url      string
	icon     string
	children []menuItem
	order    int
}

// erg.edu.vn — main educational portal
var ergMenu = []menuItem{
	{label: "Trang chủ", url: "/", order: 1},
	{label: "Giới thiệu", url: "/gioi-thieu", order: 2,
		children: []menuItem{
			{label: "Tầm nhìn & Sứ mệnh", url: "/gioi-thieu/tam-nhin-su-menh", order: 1},
			{label: "Đội ngũ", url: "/gioi-thieu/doi-ngu", order: 2},
			{label: "Liên hệ", url: "/gioi-thieu/lien-he", order: 3},
		},
	},
	{label: "Khóa học", url: "/khoa-hoc", order: 3,
		children: []menuItem{
			{label: "Tin học Quốc gia", url: "https://tinhocquocgia.erg.edu.vn", order: 1},
			{label: "Tin học Quốc tế", url: "https://tinhocquocte.erg.edu.vn", order: 2},
			{label: "AI & Lập trình", url: "/khoa-hoc/ai-lap-trinh", order: 3},
		},
	},
	{label: "Tin tức", url: "/tin-tuc", order: 4},
	{label: "Tuyển dụng", url: "https://tuyendung.erg.edu.vn", order: 5},
	{label: "Liên hệ", url: "/lien-he", order: 6},
}

// ai.erg.edu.vn — AI features subdomain
var aiMenu = []menuItem{
	{label: "Trang chủ", url: "/", order: 1},
	{label: "Tạo bài kiểm tra", url: "/tao-bai-kiem-tra", icon: "quiz", order: 2},
	{label: "Tạo kế hoạch giảng dạy", url: "/tao-ke-hoach-giang-day", icon: "lesson", order: 3},
	{label: "Phân tích học tập", url: "/phan-tich-hoc-tap", icon: "analytics", order: 4},
	{label: "Tài nguyên", url: "/tai-nguyen", order: 5,
		children: []menuItem{
			{label: "Bài mẫu", url: "/tai-nguyen/bai-mau", order: 1},
			{label: "Hướng dẫn sử dụng", url: "/tai-nguyen/huong-dan", order: 2},
		},
	},
}

// tinhocquocte.erg.edu.vn — international CS curriculum
var internationalMenu = []menuItem{
	{label: "Trang chủ", url: "/", order: 1},
	{label: "Chương trình", url: "/chuong-trinh", order: 2,
		children: []menuItem{
			{label: "IGCSE Computer Science", url: "/chuong-trinh/igcse", order: 1},
			{label: "A-Level Computer Science", url: "/chuong-trinh/alevel", order: 2},
			{label: "IB Computer Science", url: "/chuong-trinh/ib-cs", order: 3},
		},
	},
	{label: "Đăng ký", url: "/dang-ky", order: 3},
	{label: "Giảng viên", url: "/giang-vien", order: 4},
	{label: "Cảm nhận học viên", url: "/cam-nhan", order: 5},
}

// tuyendung.erg.edu.vn — recruitment subdomain
var recruitmentMenu = []menuItem{
	{label: "Trang chủ", url: "/", order: 1},
	{label: "Vị trí tuyển dụng", url: "/vi-tri", order: 2},
	{label: "Giới thiệu ERG", url: "/ve-erg", order: 3},
	{label: "Quyền lợi", url: "/quyen-loi", order: 4},
	{label: "Nộp hồ sơ", url: "/nop-ho-so", order: 5},
}

// fallbackMenu is used when domain is not recognized.
var fallbackMenu = []menuItem{
	{label: "Trang chủ", url: "/", order: 1},
	{label: "Liên hệ", url: "/lien-he", order: 2},
}

// ─── MenuService ───────────────────────────────────────────────────────────────

// Service provides navigation menu data per domain.
type Service struct {
	log *logger.Logger
}

// NewService creates a new menus service.
func NewService(log *logger.Logger) *Service {
	return &Service{log: log}
}

// getMenuForDomain returns the menu tree for the given domain.
func (s *Service) getMenuForDomain(domain string) []menuItem {
	domain = strings.ToLower(strings.TrimSpace(domain))
	switch {
	case strings.Contains(domain, "ai."):
		return aiMenu
	case strings.Contains(domain, "tinhocquocte"):
		return internationalMenu
	case strings.Contains(domain, "tuyendung"):
		return recruitmentMenu
	case strings.Contains(domain, "tinhocquocgia"):
		// National CS uses minimal menu — links back to main
		return []menuItem{
			{label: "Trang chủ", url: "/", order: 1},
			{label: "Đăng ký thi", url: "/dang-ky-thi", order: 2},
			{label: "Tra cứu chứng chỉ", url: "/tra-cuu", order: 3},
			{label: "Quy chế thi", url: "/quy-che", order: 4},
		}
	case strings.Contains(domain, "erg.edu.vn"):
		return ergMenu
	default:
		return fallbackMenu
	}
}

// GetMenuStructure returns the full menu for a given domain.
func (s *Service) GetMenuStructure(domain string) MenuStructure {
	return MenuStructure{
		Domain: domain,
		Items:  toMenuItems(s.getMenuForDomain(domain)),
	}
}

// toMenuItems converts internal menuItem to exported MenuItem.
func toMenuItems(items []menuItem) []MenuItem {
	out := make([]MenuItem, len(items))
	for i, item := range items {
		out[i] = MenuItem{
			Label:    item.label,
			URL:      item.url,
			Icon:     item.icon,
			Order:    item.order,
			Children: toMenuItems(item.children),
		}
	}
	return out
}

// ─── MenuItem (exported types) ────────────────────────────────────────────────

// MenuItem represents a single navigation item.
type MenuItem struct {
	Label    string     `json:"label"`
	URL      string     `json:"url"`
	Icon     string     `json:"icon,omitempty"`
	Children []MenuItem `json:"children,omitempty"`
	Order    int        `json:"order"`
}

// MenuStructure holds the full menu tree for a domain.
type MenuStructure struct {
	Domain string     `json:"domain"`
	Items  []MenuItem `json:"items"`
}
