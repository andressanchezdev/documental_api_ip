package document

import (
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"doc-api/core/auth"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(c echo.Context) error {
	docs, err := h.svc.List()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, docs)
}

func (h *Handler) Upload(c echo.Context) error {
	claims := c.Get("user").(*auth.CustomClaims)

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "El archivo .doc es requerido")
	}

	doc, err := h.svc.Upload(c.Request().Context(), file, claims.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, doc)
}

func (h *Handler) Preview(c echo.Context) error {
	docID, err := parseID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "ID inválido")
	}

	stream, filename, err := h.svc.GetPreview(c.Request().Context(), docID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	defer stream.Close()

	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set("Content-Disposition", "inline; filename=\""+filename+".pdf\"")
	c.Response().WriteHeader(http.StatusOK)

	_, err = io.Copy(c.Response().Writer, stream)
	return err
}

func (h *Handler) GetContent(c echo.Context) error {
	docID, err := parseID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "ID inválido")
	}

	res, err := h.svc.GetContent(c.Request().Context(), docID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return c.JSON(http.StatusOK, res)
}

func (h *Handler) SaveContent(c echo.Context) error {
	docID, err := parseID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "ID inválido")
	}

	var input UpdateContentInput
	if err := c.Bind(&input); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Datos inválidos")
	}

	if err := h.svc.UpdateContent(c.Request().Context(), docID, input.Content); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, echo.Map{"message": "Documento actualizado correctamente"})
}

func parseID(param string) (uint, error) {
	id, err := strconv.ParseUint(param, 10, 32)
	return uint(id), err
}