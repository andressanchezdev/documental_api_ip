package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(c echo.Context) error {
	var input LoginInput
	if err := c.Bind(&input); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Datos de entrada inválidos")
	}

	res, err := h.svc.Login(input)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	return c.JSON(http.StatusOK, res)
}

func (h *Handler) ListUsers(c echo.Context) error {
	users := h.svc.ListUsers()
	return c.JSON(http.StatusOK, users)
}
