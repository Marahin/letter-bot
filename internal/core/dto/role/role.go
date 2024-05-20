package role

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Permissions int64  `json:"permissions,string"`
}
