// Package templates provides notification message templates in Vietnamese and English.
package templates

import (
	"bytes"
	"strings"
	"text/template"
)

// TemplateData holds the data passed into every notification template.
type TemplateData map[string]string

// Renderer renders Go templates with custom functions for Vietnamese text.
type Renderer struct {
	funcs template.FuncMap
}

// NewRenderer creates a new template renderer with built-in functions.
func NewRenderer() *Renderer {
	return &Renderer{
		funcs: template.FuncMap{
			"bold":   bold,
			"italic": italic,
			"join":   strings.Join,
			"nl":     func() string { return "\n" },
		},
	}
}

// Render evaluates the named template with the given data.
// Templates are stored as package-level variables.
func (r *Renderer) Render(name string, data TemplateData) (string, error) {
	tmpl, ok := registry[name]
	if !ok {
		return "", &ErrTemplateNotFound{Name: name}
	}

	t, err := template.New(name).Funcs(r.funcs).Parse(tmpl)
	if err != nil {
		return "", &ErrTemplateParse{Name: name, Cause: err}
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", &ErrTemplateRender{Name: name, Cause: err}
	}
	return buf.String(), nil
}

// Register stores a named template string for use with Render.
func Register(name, tmpl string) {
	registry[name] = tmpl
}

// ─── Template registry ────────────────────────────────────────────────────────

var registry = map[string]string{
	// ── Crawl notifications ──────────────────────────────────────────────────
	"crawl_success": crawlSuccess,
	"crawl_failed":  crawlFailed,
	"crawl_started": crawlStarted,

	// ── Trending notifications ─────────────────────────────────────────────
	"hot_topic_alert":  hotTopicAlert,
	"trending_summary": trendingSummary,

	// ── System notifications ────────────────────────────────────────────────
	"system_alert":    systemAlert,
	"system_warning":  systemWarning,
	"system_recovery": systemRecovery,

	// ── Queue notifications ─────────────────────────────────────────────────
	"queue_status":   queueStatus,
	"queue_overload": queueOverload,

	// ── RSS notifications ───────────────────────────────────────────────────
	"rss_added":       rssAdded,
	"rss_removed":     rssRemoved,
	"rss_fetch_error": rssFetchError,

	// ── Account notifications ──────────────────────────────────────────────
	"account_linked":   accountLinked,
	"account_unlinked": accountUnlinked,

	// ── Generic ─────────────────────────────────────────────────────────────
	"generic": generic,
}

// ─── Vietnamese Templates ─────────────────────────────────────────────────────

var crawlSuccess = `Crawl thành công!
📰 Tiêu đề: {{.title}}
🌐 URL: {{.url}}
⏱ Thời gian: {{.duration}}
📊 Điểm chất lượng: {{.quality_score}}`

var crawlFailed = `Crawl thất bại!
🌐 URL: {{.url}}
⚠️ Lý do: {{.error}}
🔄 Thử lại tự động lúc: {{.retry_at}}
📋 Job ID: {{.job_id}}`

var crawlStarted = `🔍 Crawl đã bắt đầu
🌐 URL: {{.url}}
⏱ Ước tính: {{.estimated_time}}
📋 Job ID: {{.job_id}}
🔗 Theo dõi: {{.track_url}}`

var hotTopicAlert = `🔥 Topic hot: {{.topic}}
📊 Khối lượng: {{.volume}}
📈 Xu hướng: {{.trend_direction}}
🔗 {{.url}}
⏱ Cập nhật: {{.updated_at}}`

var trendingSummary = `📬 Trending Daily — {{.date}}
Bài viết nổi bật hôm nay:
{{range .items}}• {{.}}
{{end}}
🔗 Xem thêm: {{.dashboard_url}}`

var systemAlert = `⚠️ Cảnh báo hệ thống
{{.alert_type}}
{{.message}}
🕐 Thời gian: {{.timestamp}}
🔧 Xem chi tiết: {{.admin_url}}`

var systemWarning = `⚡ Cảnh báo: {{.title}}
{{.message}}
📊 {{.metric_name}}: {{.metric_value}}
🔧 {{.admin_url}}`

var systemRecovery = `✅ Hệ thống đã khôi phục
{{.title}}
⏱ Khôi phục lúc: {{.recovered_at}}
📊 Downtime: {{.downtime}}`

var queueStatus = `📊 Queue Status — {{.queue_name}}
⏳ Pending: {{.pending}}
⚡ Processing: {{.processing}}/min
❌ Failed: {{.failed}}
📦 Workers: {{.workers}}`

var queueOverload = `🚨 Queue overload!
📊 Depth: {{.depth}} (max {{.max_depth}})
⏱ Avg wait: {{.avg_wait}}
🔧 {{.admin_url}}`

var rssAdded = `✅ RSS feed đã thêm!
📡 Tên: {{.feed_name}}
🔗 URL: {{.feed_url}}
📅 Tần suất: {{.frequency}}`

var rssRemoved = `🗑 RSS feed đã xóa
📡 {{.feed_name}}
🔗 {{.feed_url}}`

var rssFetchError = `⚠️ Lỗi fetch RSS
📡 {{.feed_name}}
🔗 {{.feed_url}}
⚠️ {{.error}}`

var accountLinked = `✅ Tài khoản đã liên kết thành công!
🏠 Nền tảng: {{.platform}}
👤 {{.platform_user}}
🕐 Lúc: {{.linked_at}}`

var accountUnlinked = `🔓 Tài khoản đã hủy liên kết
🏠 Nền tảng: {{.platform}}
👤 {{.platform_user}}`

var generic = `{{.subject}}
{{.body}}
⏱ {{.timestamp}}`

// ─── Error types ─────────────────────────────────────────────────────────────

// ErrTemplateNotFound is returned when a template name is not registered.
type ErrTemplateNotFound struct {
	Name string
}

func (e *ErrTemplateNotFound) Error() string {
	return "template not found: " + e.Name
}

// ErrTemplateParse is returned when a template fails to parse.
type ErrTemplateParse struct {
	Name  string
	Cause error
}

func (e *ErrTemplateParse) Error() string {
	return "template parse error (" + e.Name + "): " + e.Cause.Error()
}

// ErrTemplateRender is returned when a template fails to execute.
type ErrTemplateRender struct {
	Name  string
	Cause error
}

func (e *ErrTemplateRender) Error() string {
	return "template render error (" + e.Name + "): " + e.Cause.Error()
}

// ─── Helper template functions ───────────────────────────────────────────────

func bold(s string) string   { return "**" + s + "**" }
func italic(s string) string { return "_" + s + "_" }
