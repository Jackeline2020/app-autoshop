package dto

type CreateServiceRequest struct {
	Name          string  `json:"name"           binding:"required"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"          binding:"required"`
	EstimatedTime int     `json:"estimated_time" binding:"required"`
}

type UpdateServiceRequest struct {
	Name          string  `json:"name"           binding:"required"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"          binding:"required"`
	EstimatedTime int     `json:"estimated_time" binding:"required"`
}
