package dto

type CreateVehicleRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	Plate      string `json:"plate"       binding:"required"`
	Brand      string `json:"brand"       binding:"required"`
	Model      string `json:"model"       binding:"required"`
	Year       int    `json:"year"        binding:"required"`
}

type UpdateVehicleRequest struct {
	Plate string `json:"plate" binding:"required"`
	Brand string `json:"brand" binding:"required"`
	Model string `json:"model" binding:"required"`
	Year  int    `json:"year"  binding:"required"`
}
