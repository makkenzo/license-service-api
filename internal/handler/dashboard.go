package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/makkenzo/license-service-api/internal/service"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	licenseService *service.LicenseService
	logger         *zap.Logger
}

func NewDashboardHandler(licenseService *service.LicenseService, logger *zap.Logger) *DashboardHandler {
	return &DashboardHandler{
		licenseService: licenseService,
		logger:         logger.Named("DashboardHandler"),
	}
}

// GetSummary godoc
// @Summary      Get dashboard summary
// @Description  Retrieves aggregated statistics about licenses for the dashboard.
// @Tags         dashboard
// @Accept       json
// @Produce      json
// @Success      200 {object} dto.DashboardSummaryResponse "Dashboard summary data"
// @Failure      500 {object} map[string]string "Internal Server Error"
// @Router       /dashboard/summary [get]
func (h *DashboardHandler) GetSummary(c *gin.Context) {
	h.logger.Info("Received request for dashboard summary")

	summary, err := h.licenseService.GetDashboardSummary(c.Request.Context())
	if err != nil {

		h.logger.Error("Failed to get dashboard summary from service", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dashboard summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}
