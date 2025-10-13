package service

import (
	"context"
	"log/slog"
	"mymodule/internal/domains"
)

type TemplateService struct {
	provider TemplateProvider
	secret   string
}

type TemplateProvider interface {
	SaveTemplate(ctx context.Context, template domains.TemplateCreate, userId int) error
	GetAllTemplatesByUser(ctx context.Context, userId int) ([]domains.Template, error)
}

func NewTemplateService(provider TemplateProvider) *TemplateService {
	return &TemplateService{
		provider: provider,
	}
}

func (h *TemplateService) CreateTemplate(ctx context.Context, template domains.TemplateCreate, userId int) error {
	err := h.provider.SaveTemplate(ctx, template, userId)
	if err != nil {
		slog.Error("Save template error", err.Error())
		return err
	}

	return nil
}

func (h *TemplateService) GetAllTemplatesByUser(ctx context.Context, userId int) ([]domains.Template, error) {
	templates, err := h.provider.GetAllTemplatesByUser(ctx, userId)
	if err != nil {
		slog.Error("Get template error", err.Error())
		return nil, err
	}
	return templates, nil
}
