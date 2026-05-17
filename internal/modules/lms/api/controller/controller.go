package controller

import (
	lmsservice "erg.ninja/internal/modules/lms/application/service"
)

type Controller = lmsservice.Controller

func NewController(svc *lmsservice.Service) *Controller {
	return lmsservice.NewController(svc)
}
