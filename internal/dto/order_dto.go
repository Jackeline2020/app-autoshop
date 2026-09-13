package dto

type OrderServiceRequest struct {
	ServiceID string `json:"service_id" binding:"required"`
}

type OrderPartRequest struct {
	PartID   string `json:"part_id"  binding:"required"`
	Quantity int    `json:"quantity" binding:"required"`
}

type CreateOrderRequest struct {
	CustomerID string                `json:"customer_id" binding:"required"`
	VehicleID  string                `json:"vehicle_id"  binding:"required"`
	Services   []OrderServiceRequest `json:"services"   binding:"required"`
	Parts      []OrderPartRequest    `json:"parts"`
	Notes      string                `json:"notes"`
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type ApproveOrderRequest struct {
	Approved *bool  `json:"approved" binding:"required"`
	Reason   string `json:"reason"` // motivo caso recuse
}

type OrderStatusResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	StatusLabel string `json:"status_label"`
	UpdatedAt   string `json:"updated_at"`
}

type EmailStatusUpdateRequest struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Subject string `json:"subject"`
	From    string `json:"from"`
}
