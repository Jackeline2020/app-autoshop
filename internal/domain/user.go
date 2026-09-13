package domain

type User struct {
	ID       string `json:"id"       dynamodbav:"id"`
	Name     string `json:"name"     dynamodbav:"name"`
	Email    string `json:"email"    dynamodbav:"email"`
	Password string `json:"password" dynamodbav:"password"`
	Role     string `json:"role"     dynamodbav:"role"` // admin, mechanic
}
