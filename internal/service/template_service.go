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
	UpdateTemplate(ctx context.Context, templateId int, template domains.TemplateCreate, userId int) error
	GetAllTemplatesByUser(ctx context.Context, userId int) ([]domains.Template, error)
	GetTemplateById(ctx context.Context, userId int, templateId int) (domains.Template, error)
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

func (h *TemplateService) GetTemplateById(ctx context.Context, userId int, templateId int) (domains.Template, error) {
	slog.Info("get template by id", userId, templateId)
	template, err := h.provider.GetTemplateById(ctx, userId, templateId)
	if err != nil {
		slog.Error("Get template error", err.Error())
		return domains.Template{}, err
	}
	return template, nil
}

func (h *TemplateService) UpdateTemplate(ctx context.Context, templateIDInt int, template domains.TemplateCreate, userId int) error {
	err := h.provider.UpdateTemplate(ctx, templateIDInt, template, userId)
	if err != nil {
		slog.Error("Save template error", err.Error())
		return err
	}

	return nil
}
