package watermark

import (
	"fmt"
	"io"

	entities "erg.ninja/internal/modules/documents/domain/entity"
	"erg.ninja/pkg/logger"
)

// Renderer applies text watermarks to PDF byte slices using pdfcpu.
type Renderer struct {
	log *logger.Logger
}

// NewRenderer creates a new watermark renderer.
func NewRenderer(log *logger.Logger) *Renderer {
	return &Renderer{log: log}
}

// Position constants.
const (
	PositionCenter = "CENTER"
	PositionCorner = "CORNER"
	PositionTiled  = "TILED"
)

// Apply applies a watermark to a PDF byte slice and returns the watermarked PDF.
func (r *Renderer) Apply(pdfBytes []byte, cfg entities.WatermarkConfig) ([]byte, error) {
	if cfg.Text == "" {
		return pdfBytes, nil
	}
	r.log.Warn().
		Str("text", cfg.Text).
		Str("position", cfg.Position).
		Float64("opacity", cfg.Opacity).
		Str("color", cfg.Color).
		Int("font_size", cfg.FontSize).
		Float64("offset_x", cfg.OffsetX).
		Float64("offset_y", cfg.OffsetY).
		Float64("rotation", cfg.Rotation).
		Msg("watermark: renderer dependency unavailable")

	return nil, fmt.Errorf("watermark renderer dependency unavailable")
}

// ValidateConfig returns an error if the watermark config is invalid.
func ValidateConfig(cfg entities.WatermarkConfig) error {
	if cfg.Text == "" {
		return fmt.Errorf("watermark text is required")
	}
	switch cfg.Position {
	case PositionCenter, PositionCorner, PositionTiled:
	default:
		return fmt.Errorf("invalid position %q (want CENTER|CORNER|TILED)", cfg.Position)
	}
	if cfg.Opacity < 0 || cfg.Opacity > 1 {
		return fmt.Errorf("opacity must be between 0 and 1")
	}
	if cfg.FontSize < 8 || cfg.FontSize > 144 {
		return fmt.Errorf("font_size must be between 8 and 144")
	}
	if cfg.OffsetX < 0 || cfg.OffsetX > 100 {
		return fmt.Errorf("offset_x must be between 0 and 100")
	}
	if cfg.OffsetY < 0 || cfg.OffsetY > 100 {
		return fmt.Errorf("offset_y must be between 0 and 100")
	}
	if cfg.Rotation < -90 || cfg.Rotation > 90 {
		return fmt.Errorf("rotation must be between -90 and 90")
	}
	return nil
}

// ProcessStream applies watermark to a stream on the fly.
func (r *Renderer) ProcessStream(src io.Reader, dst io.Writer, cfg entities.WatermarkConfig) error {
	pdfBytes, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	wmPDF, err := r.Apply(pdfBytes, cfg)
	if err != nil {
		return err
	}

	_, err = dst.Write(wmPDF)
	return err
}
