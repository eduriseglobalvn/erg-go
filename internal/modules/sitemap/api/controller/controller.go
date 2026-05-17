package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	sitemapservice "erg.ninja/internal/modules/sitemap/application/service"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// Controller handles sitemap rendering endpoints.
type Controller struct {
	svc *sitemapservice.Service
	log *logger.Logger
	cfg *config.Config
}

// NewController creates a new sitemap controller.
func NewController(svc *sitemapservice.Service, log *logger.Logger, cfg *config.Config) *Controller {
	return &Controller{svc: svc, log: log, cfg: cfg}
}

// RegisterRoutes mounts the sitemap/robots endpoints.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	// These routes are mounted at the root generally, but we can put them in a group if needed.
	// We'll stick to root level to match typical usage (GET /robots.txt, GET /sitemap.xml).

	r.GET("/robots.txt", c.GetRobotsTxt)
	r.GET("/sitemap_index.xml", c.GetSitemapIndex)
	r.GET("/sitemap.xml", c.GetSitemap)

	// Add support for data endpoints or subdomain sitemaps if required
	r.GET("/sitemap/data", c.GetSitemapData) // usually internal API use
	r.GET("/sitemap-:sub.xml", c.GetSubSitemap)
}

// getPublicDomain fetches the configured base URL
func (c *Controller) getPublicDomain() string {
	domain := fmt.Sprintf("http://%s:%d", c.cfg.App.Host, c.cfg.App.Port)
	if c.cfg.App.Host == "0.0.0.0" {
		domain = fmt.Sprintf("http://localhost:%d", c.cfg.App.Port)
	}
	return strings.TrimSuffix(domain, "/")
}

// GetRobotsTxt returns the robots.txt file.
// @Summary Get robots.txt
// @Description Returns the robots.txt file.
// @Tags Sitemap
// @Produce text/plain
// @Success 200
// @Router /robots.txt [get]
func (c *Controller) GetRobotsTxt(ctx *gin.Context) {
	data, err := c.svc.GenerateRobotsTxt(c.getPublicDomain())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("sitemap: GenerateRobotsTxt failed")
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

// GetSitemapIndex returns the sitemap index.
// @Summary Get sitemap index
// @Description Returns the XML sitemap index.
// @Tags Sitemap
// @Produce application/xml
// @Success 200
// @Router /sitemap_index.xml [get]
func (c *Controller) GetSitemapIndex(ctx *gin.Context) {
	data, err := c.svc.GenerateSitemapIndex(ctx.Request.Context(), c.getPublicDomain())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("sitemap: GenerateSitemapIndex failed")
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	ctx.Data(http.StatusOK, "application/xml; charset=utf-8", data)
}

// GetSitemap returns the main sitemap XML.
// @Summary Get sitemap
// @Description Returns the main sitemap XML.
// @Tags Sitemap
// @Produce application/xml
// @Success 200
// @Router /sitemap.xml [get]
func (c *Controller) GetSitemap(ctx *gin.Context) {
	data, err := c.svc.GenerateSitemap(ctx.Request.Context(), c.getPublicDomain())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("sitemap: GenerateSitemap failed")
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	ctx.Data(http.StatusOK, "application/xml; charset=utf-8", data)
}

// GetSubSitemap placeholder for /sitemap-:sub.xml
// @Summary Get sub-sitemap
// @Description Returns a sub-sitemap partition.
// @Tags Sitemap
// @Produce application/xml
// @Param sub path string true "Sub partition"
// @Success 200
// @Router /sitemap-{sub}.xml [get]
func (c *Controller) GetSubSitemap(ctx *gin.Context) {
	// For now, return the same logic as base sitemap or specifically filtered.
	sub := ctx.Param("sub")
	_ = sub

	data, err := c.svc.GenerateSitemap(ctx.Request.Context(), c.getPublicDomain())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("sitemap: GenerateSubSitemap failed")
		ctx.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	ctx.Data(http.StatusOK, "application/xml; charset=utf-8", data)
}

// GetSitemapData returns sitemap items in JSON format, if needed.
// @Summary Get sitemap data (JSON)
// @Description Returns sitemap items in JSON format.
// @Tags Sitemap
// @Produce json
// @Success 200 {object} map[string]any
// @Router /sitemap/data [get]
func (c *Controller) GetSitemapData(ctx *gin.Context) {
	response.WriteGin(ctx, http.StatusOK, gin.H{"status": "ok", "message": "sitemap data api placeholder"}, nil, nil)
}
